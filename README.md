## syncthing-relay-reporter

A simple program to report Syncthing relay usage statistics to be shown in Grafana. 

# Features
- Pulls Syncthing relay usage data from the relays private/public data port
- Uploads the data to InfluxDB for processing
- Support for InfluxDB authentication with an username and password
- Supported data from the relay
    - Bytes proxied
    - Uptime
    - Network usage rates
        - 10s
        - 1m
        - 5m
        - 15m
        - 30m
        - 60m
    - Active session count
    - Connection count
    - Pending session count
    - Proxy count
- Dashboard template supports multiple relays

# Installation
1. Have an accessible InfluxDB instance to the reporter
2. Create a new database in InfluxDB to store the data, use the same name in the options
3. Have an accessible Syncthing relay data port to the reporter
4. Build (install) the reporter
    - Build with ```go build``` or install with ```go install```
5. Run the reporter with atleast the required options
    - Required
        - relay-url
        - influxdb-url
        - influxdb-database
    - Optional
        - influxdb-username
        - influxdb-password
        - relay-name (used to differentiate between multiple relays in the database/dashboard, default is default-relay)
        - debug (true/false for more verbose debug output, default is false)

# Running

Use your preferred init system.

# Grafana dashboard
1. Add a new InfluxDB datasource with the database name you ran the reporter with
3. Create a new dashboard
3. Copy the grafana-dashboard.json file into your grafana dashboard settings
4. Replace your database name in the json file. (Every needed place for the name is DATASOURCE_HERE)

# The dashboard template in use
![Image of the Grafana dashboard template is use](https://i.imgur.com/nva84Is.png)