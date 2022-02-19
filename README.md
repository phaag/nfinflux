# Nfdump InfluxDB Exporter

This is a prototype exporter for nfdump. It sends periodic updates to an InfluxDB.

It's purpose is to play and experiment with nfdump netflow data and InfluxDB/Grafana to build a new graphical UI as a repacement for aging NfSen.

This experimental exporter exposes counters for flows/packets and bytes per protocol (tcp/udp/icmp/other) and the source identifier from the nfcapd collector. Updates are sent every minute (60s). Multiple collectors (ident) with multiple exporters each may send metrics to the exporter.

## Metrics:

For **flows**, **packets** and **bytes** and each protocol **tcp**, **udp**, **icmp** and **other** a metric record is added in the form of:

```
p := influxdb2.NewPointWithMeasurement("stat").
	AddTag("collector", "nfcapd").
	AddTag("version", "1.7-beta").
	AddTag("ident", ident).
	AddTag("exporterID", strconv.FormatUint(metric.exporterID, 10)).
	AddTag("proto", PROTO).
	AddField(RATE, metric.numFlows_PROTO).
	SetTime(now)
	writeAPI.WritePoint(p)
```

With PROTO: **tcp**, **udp**, **icmp**, **other** and RATE: **tcp**, **udp**, **icmp** and **other**. A total of 12 points are written for the same timestamp.

## Usage:

```
Usage of ./nfdump_influx:
  -UNIX socket string
        Path for nfcapd collectors to connect (default "/tmp/nfsen.sock")
  -influxDB bucket string
        influxDB bucket name (default "NfSen")
  -influxDB host string
        Address to send telemetry data (default "127.0.0.1:8086")
  -influxDB organisation string
        influxDB organisation name (default "Netflow")
  -influxDB token string
        influxDB token (default "-")

```

The nfdump_influx listens on a UNIX socket for statistics sent by the nfcapd collector.

Setup an InfluxDB and create a toke for nfdump_influx. Run nfdump_influx with the specific parameters.

## Nfdump

The metric export is integrated in nfdump 1.7-beta

In order not to pollute an existing nfdump netflow installation, forward the traffic from an existing collector. Add: `-R 127.0.0.1/9999` to the argument list and setup the new collector. You may also send it to another host, which runs also Prometheus for example.

Build nfdump 1.7-beta:

`git clone -b unicorn https://github.com/phaag/nfdump.git nfdump.unicorn`

Build nfdump with `sh bootstrap.sh; ./configure` but do not run make install, as it would replace your existing installation. Create a tmp flow dir and run the collector from the src directory. For example:

`./nfcapd -l <tmpflows> -S2 -y -p 9999 -m <metric socket>`

If adding `-m <metric socket>` nfcapd exports the internal statistics every 5s the the exporter.

## Note:

Only the statistics values are exposed and not the netflow records itself.
