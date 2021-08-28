# Nfdump InfluxDB Exporter

This is a prototype exporter for nfdump. It sends periodic updates to an InfluxDB.

It's purpose is to play and experiment with nfdump netflow data and InfluxDB/Grafana to build a new graphical UI as a repacement for aging NfSen.

This experimental exporter exposes counters for flows/packets and bytes per protocol (tcp/udp/icmp/other) and the source identifier from the nfcapd collector. (currently hardwired "live"). Updates are sent every 30s.

## Metrics:

```
    AddTag("collector", "nfcapd").
    AddTag("version", "1.7-beta").
    AddTag("ident", metric.ident).
    AddTag("exporterID", exporterID).
    AddField("flows_tcp", metric.numFlows_tcp).
    AddField("flows_udp", metric.numFlows_udp).
    AddField("flows_icmp", metric.numFlows_icmp).
    AddField("flows_other", metric.numFlows_other).
    AddField("packets_tcp", metric.numPackets_tcp).
    AddField("packets_udp", metric.numPackets_udp).
    AddField("packets_icmp", metric.numPackets_icmp).
    AddField("packets_other", metric.numPackets_other).
    AddField("bytes_tcp", metric.numBytes_tcp).
    AddField("bytes_udp", metric.numBytes_udp).
    AddField("bytes_icmp", metric.numBytes_icmp).
    AddField("bytes_other", metric.numBytes_other).
    SetTime(time.Now())
```



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
