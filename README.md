# nfinflux - an nfdump InfluxDB interface

**nfinflux** is a data interface between [nfdump](https://github.com/phaag/nfdump/tree/unicorn) and influxDB.

nfinflux has two operation modes:

1. Continous mode:
   Creates a UNIX sockets and waits for metric data being sent by any number of nfcapd, sfcapd or nfpcapd collectors.
2. Import mode:
   Take any number of pre-collected nfcapd files and imports the stat info into influxDB

In both modes, the same type of data is imported. See below for a more details

The collector exposes rate counters for flows/packets and bytes per protocol (tcp/udp/icmp/other) and the source identifier from the respective exporter. Updates are sent by default every minute (60s).  Multiple collectors (ident) with multiple exporters may send metrics to **nfinflux**.

## Metrics:

For each protocol **tcp**, **udp**, **icmp** and **other** a metric record is added in the form of:

```
// Example: create point proto tcp
	p := write.NewPoint(
		"stat",
		map[string]string{
			"channel" : ident,
			"exporter": exporterID,
			"proto"   : "tcp",
		},
		map[string]interface{}{
			"fps": statRecord.NumflowsTcp,
			"pps": statRecord.NumpacketsTcp,
			"bps": statRecord.NumbytesTcp,
		},
		when)
	writeAPI.WritePoint(p)
```

The metric records contains the rates/s for flows, packets and bytes.  4 points are written for the same timestamp.

## Installation:

nfinflux is written in golang. Make sure you have at least golang 1.17 installed on your system. 

1. Download the master branch.
2. go mod tidy
3. go build

nfinflux is built in the current directory.

## Usage:

```
Usage of ./nfinflux:
  -bucket string
    	influxDB bucket name (default "life")
  -create
    	create bucket, if it does not exist
  -delete
    	delete existing bucket first
  -host string
    	Address to send metric data (default "http://127.0.0.1:8086")
  -org string
    	influxDB organisation name (default "Netflow")
  -socket string
    	Path for nfcapd collectors to connect
  -token string
    	influxDB token (default "-")
  -twin int
    	time interval in seconds of flow file (default 300)
```

Continous mode: If **-socket** is given, the continous mode is active. It opens the requested socket and listen for incoming messages, which are are converted into influxDB points ant sent to the InfluxDB.

Import mode: If any **files** and/or **directories** are given as extra arguments, nfinflux runs in import mode and imports the stat records of any nfcapd files found recursively in directories. It ends after the successful import.

### Examples:

Continous mode:

````
./nfinflux -socket /tmp/nfdump -host http://127.0.0.1:8086 -org MyOrg -bucket Flows -token <token>

./nfcapd -l /flowdir -S2 -y -m /tmp/nfdump
./sfcapd -l /sflowdir -S2 -y -m /tmp/nfdump
````

Runs nfinflux and nfcapd, sfcapd to collect and import continously metric data.

Import mode:

```
./nfinflux -host http://127.0.0.1:8086 -org MyOrg -bucket Flows -token <token> nfcapd.202203030015
./nfinflux -host http://127.0.0.1:8086 -org MyOrg -bucket Flows -token <token> /flowdir/2022
```

Imports the stats of a single netflow file or recursively all netflow files in /flowdir/2022. Multiple files or directories may be given as extra arguments. 
Usually nfcapd.xx files are collected each 300s interval. The timestamp is taken from the file name and the rates calculated by assuming a 300s interval. If you collected your flows in a different interval, add the proper **-twin** option.

### InfluxDB

nfinflux uses the InfluxDB api v2.0, therefore requires an InfluxDB version >= v2.0.

For any operation mode, valid credentials (host/token/org) for the InfluxDB are required. nfinflux also supports environment variables **INFLUXDB_HOST** and **INFLUXDB_TOKEN**. The **-host** and **-token** command line arguments overwrite the env variables.

#### Bucket

Alle data is imported into the bucket given with **-bucket**. There are two additional command line options:

**-create**: Creates the bucket if it does not exist.
**-delete**: Deletes the bucket if it exists and re-creates it.

Both options need a token with appropriate authorization rights. (Allow read/write for given org). Otherwise a token to write/write the bucket is fine.

A proper token needs to be created in the InfluxDB interface. The organisation needs already to exist.

## Nfdump

The metric export is integrated in [nfdump 1.7-beta](https://github.com/phaag/nfdump/tree/unicorn) in the unicorn branch and works for all collectors nfcapd, sfcapd and nfpcapd. Metrics are exported per identifier (./nfcapd -I <ident>) and exporter. Multiple exporters generate multiple metrices.

Build nfdump 1.7-beta:

`git clone -b unicorn https://github.com/phaag/nfdump.git nfdump.unicorn`

Build nfdump with `sh bootstrap.sh; ./configure` but do not run make install, as it would replace your existing installation. Create a tmp flow dir and run the collector from the src directory. For example:

`./nfcapd -l <tmpflows> -S2 -y -p 9999 -m <metric socket>`

If adding `-m <metric socket>` nfcapd exports the internal statistics by exporter every 60s by default. The argument -i <interval>  may be used to change to interval

## Note:

Only the statistics values are exposed. These are the rate values flows/s (fps), packets/s (ops) and bits/s (bps) within the interval. No netflow record content is exported.
