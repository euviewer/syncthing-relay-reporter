package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func debug_loggging(msg string) {
	fmt.Println("[DEBUG]", time.Now().Format(time.RFC3339), msg)
}

func logging(msg string) {
	fmt.Println("[INFO]", time.Now().Format(time.RFC3339), msg)
}

func error_logging(msg string) {
	fmt.Println("[ERROR]", time.Now().Format(time.RFC3339), msg)
}

type configuration struct {
	syncthingRelayUrl  string
	syncthingRelayName string
	influxdbUrl        string
	influxdbDatabase   string
	influxdbUsername   string
	influxdbPassword   string
	rateMultiplier     float64
	debug              bool
}

func configuration_builder() configuration {
	// Get command line arguments
	var conf configuration

	debug := flag.Bool("debug", false, "Boolean to toggle debug output mode.")
	rateMultiplier := flag.Float64("rate-multiplier", 1, "Multiplier * 1 second, how often values are fetched and uploaded to the database.")
	syncthingRelayUrl := flag.String("relay-url", "", "Required syncthing relay url to fetch updates from.")
	syncthingRelayName := flag.String("relay-name", "default-relay", "Optional syncthing relay name to make it possible to view multiple relays with a single database and/or dashboard.")
	influxdbUrl := flag.String("influxdb-url", "", "Required influxdb url to push updates to.")
	influxdbDatabase := flag.String("influxdb-database", "", "Required InfluxDB database name to write values to.")
	influxdbUsername := flag.String("influxdb-username", "", "Optional InfluxDB database username, if one is set.")
	influxdbPassword := flag.String("influxdb-password", "", "Optional InfluxDB database password, if one is set.")
	flag.Parse()

	// Additional InfluxDB url parsing, consistent base in later processing
	var url = *influxdbUrl
	if url[len(url)-1] != '/' {
		*influxdbUrl = url + "/"
	}

	conf.debug = *debug
	conf.rateMultiplier = *rateMultiplier
	conf.syncthingRelayUrl = *syncthingRelayUrl
	conf.syncthingRelayName = *syncthingRelayName
	conf.influxdbUrl = *influxdbUrl
	conf.influxdbDatabase = *influxdbDatabase
	conf.influxdbUsername = *influxdbUsername
	conf.influxdbPassword = *influxdbPassword

	// Debug the config struct
	if conf.debug {
		debug_loggging(fmt.Sprint(conf))
	}

	// Check if all the values are retrieved
	if conf.rateMultiplier == 1 {
		logging("Rate multiplier at the default value of 1! Set with the --rate-multiplier= argument.")
	}
	if conf.syncthingRelayUrl == "" {
		panic("No Syncthing relay URL found! Set with the --relay-url= argument.")
	}
	if conf.syncthingRelayName == "default-relay" {
		logging("No Syncthing relay name found! Using the default name 'default-relay'. Set with the --relay-name= argument.")
	}
	if conf.influxdbUrl == "" {
		panic("No InfluxDB URL found! Set with the --influxdb-url= argument.")
	}
	if conf.influxdbDatabase == "" {
		panic("No InfluxDB database found! Set with the --influxdb-database= argument.")
	}
	if conf.influxdbUsername == "" {
		logging("No InfluxDB username found! If needed, set with the --influxdb-username= argument.")
	}
	if conf.influxdbPassword == "" {
		logging("No InfluxDB password found! If needed, set with the --influxdb-password= argument.")
	}

	return conf
}

func conn_tester(conf *configuration) {
	logging("Starting connection tests.")

	// Load relay server status page
	response, err := http.Get(conf.syncthingRelayUrl)
	if err != nil {
		logging("Initial Syncthing relay server load failed! Is the url in the correct form: \"<https/http>://<domain/ip>:<port>/status\" ?")
		panic(err)
	}
	if response.StatusCode != 200 {
		panic("Syncthing relay server was reached but the HTTP status code was not 200! Is the url in the correct form: \"<https/http>://<domain/ip>:<port>/status\" ?")
	}
	logging("Syncthing relay connection check successful.")

	// Load InfluxDB health check
	response, err = http.Get(conf.influxdbUrl + "health")
	if err != nil {
		logging("Initial InfluxDB server load failed! Is the url in the correct form: \"<https/http>://<domain/ip>:<port>/status\" ?")
		panic(err)
	}
	if response.StatusCode != 200 {
		panic("InfluxDB server was reached but the HTTP status code was not 200! Is the url in the correct form: \"<https/http>://<domain/ip>:<port>/status\" ?")
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		error_logging("InfluxDB connection test HTTP body reading error.")
		panic(err)
	}

	var healthResponse map[string]interface{}
	err = json.Unmarshal(body, &healthResponse)
	if err != nil {
		error_logging("InfluxDB connection test HTTP body unmarshaling error.")
		panic(err)
	}

	if healthResponse["status"].(string) != "pass" {
		error_logging("InfluxDB health check was not a pass!")
	}

	logging("InfluxDB connection check successful.")
}

func reporter(conf *configuration) {
	// Get data from the relay
	response, err := http.Get(conf.syncthingRelayUrl)
	if err != nil {
		error_logging("Reporter failed fetch data from the relay!")
		error_logging(err.Error())
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		error_logging("Reporter failed to read HTTP body of the relay data!")
		error_logging(err.Error())
		return
	}
	var relayResponse map[string]interface{}
	err = json.Unmarshal(body, &relayResponse)
	if err != nil {
		error_logging("Reporter failed to unmarshal the relay data!")
		error_logging(err.Error())
		return
	}

	// Write data to the database
	parameters := url.Values{}
	parameters.Add("db", conf.influxdbDatabase)

	var relayNetworkRates []interface{} = relayResponse["kbps10s1m5m15m30m60m"].([]interface{})
	timestamp := time.Now().UnixNano()

	reqBody := fmt.Sprintf("%s,relay=%s bytesProxied=%v,uptime=%v,kbps10s=%v,kbps1m=%v,kbps5m=%v,kbps15m=%v,kbps30m=%v,kbps60m=%v,activeSessions=%v,connections=%v,pendingSessionKeys=%v,proxies=%v %d",
		conf.syncthingRelayName, conf.syncthingRelayName, relayResponse["bytesProxied"], relayResponse["uptimeSeconds"], relayNetworkRates[0], relayNetworkRates[1], relayNetworkRates[2], relayNetworkRates[3], relayNetworkRates[4], relayNetworkRates[5], relayResponse["numActiveSessions"], relayResponse["numConnections"], relayResponse["numPendingSessionKeys"], relayResponse["numProxies"], timestamp)
	bodyReader := bytes.NewReader([]byte(reqBody))

	client := &http.Client{}
	req, err := http.NewRequest("POST", conf.influxdbUrl+"write?"+parameters.Encode(), bodyReader)
	req.Header.Add("Authorization", "Token "+conf.influxdbUsername+":"+conf.influxdbPassword)

	response, err = client.Do(req)
	if err != nil {
		error_logging("InfluxDB writing HTTP request failed!")
		error_logging(err.Error())
		return
	}

	if conf.debug {
		logging("InfluxDB request body: " + reqBody)
		logging("InfluxDB full URL: " + conf.influxdbUrl + "write" + parameters.Encode())
		resBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			error_logging("Failed to read InfluxDB response body for debugging purposes!")
			error_logging(err.Error())
			return
		}
		logging("Reporter was successful.")
		logging("InfluxDB write request response body:\n" + string(resBody))
	}

}

func main() {
	// Build configuration
	globalConfiguration := configuration_builder()

	// Test connections
	conn_tester(&globalConfiguration)

	// New ticker to execute fetching values from the relay and push to influxdb
	reporterTicker := time.NewTicker(time.Duration(globalConfiguration.rateMultiplier) * time.Second)
	defer reporterTicker.Stop()
	reporterTickerDone := make(chan bool)

	// Listen to system calls, if terminated stop reporter_ticker
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		_ = <-sigc
		logging("Shutdown signal caught, shutting down gracefully.")
		reporterTickerDone <- true
	}()

	for {
		select {
		case <-reporterTickerDone:
			logging("Shutting down reporter ticker.")
			return
		case _ = <-reporterTicker.C:
			// Pull values from relay and push to database
			go reporter(&globalConfiguration)
		}
	}
}
