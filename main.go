/* 
 *  Copyright (c) 2021, Peter Haag
 *  All rights reserved.
 *  
 *  Redistribution and use in source and binary forms, with or without 
 *  modification, are permitted provided that the following conditions are met:
 *  
 *   * Redistributions of source code must retain the above copyright notice, 
 *     this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above copyright notice, 
 *     this list of conditions and the following disclaimer in the documentation 
 *     and/or other materials provided with the distribution.
 *   * Neither the name of the author nor the names of its contributors may be 
 *     used to endorse or promote products derived from this software without 
 *     specific prior written permission.
 *  
 *  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" 
 *  AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE 
 *  IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE 
 *  ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE 
 *  LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR 
 *  CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF 
 *  SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS 
 *  INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN 
 *  CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) 
 *  ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE 
 *  POSSIBILITY OF SUCH DAMAGE.
*/

/*
 * Poc to implement a metric exporter for nfcapd collectors to influxDB
 */

package main

import (
	"flag"
	"fmt"
	"sync"
	"log"
	"os"
	"os/signal"
	"time"
	"syscall"
	"strconv"

	"github.com/influxdata/influxdb-client-go/v2"
)

var mutex *sync.Mutex

var (
	influxHost = flag.String("influxDB host", "127.0.0.1:8086",
		"Address to send telemetry data")
	org = flag.String("influxDB organisation", "Netflow",
		"influxDB organisation name")
	bucket = flag.String("influxDB bucket", "NfSen",
		"influxDB bucket name")
	token = flag.String("influxDB token", "-",
		"influxDB token")
	socketPath = flag.String("UNIX socket", "/tmp/nfsen.sock",
		"Path for nfcapd collectors to connect")
)

type influxDBConf struct {
	host string
	token string
	bucket string
	org	string
}

func NewExporter(host string, org string, bucket string, token string) *influxDBConf {
	influxDB := new(influxDBConf)
    influxDB.host	= host
    influxDB.token	= token
    influxDB.bucket = bucket
    influxDB.org	= org
    return influxDB
} // End of NewExporter

func (influxDB *influxDBConf) insertStat() {
	// You can generate a Token from the "Tokens Tab" in the UI

	client := influxdb2.NewClient("http://" + influxDB.host, influxDB.token)
	// always close client at the end
	defer client.Close()

	// get non-blocking write client
	writeAPI := client.WriteAPI(influxDB.org, influxDB.bucket)

	// Get errors channel
    errorsCh := writeAPI.Errors()
    // Create go proc for reading and logging errors
    go func() {
        for err := range errorsCh {
            fmt.Printf("write error: %s\n", err.Error())
        }
    }()

	// create point using fluent style
	mutex.Lock()
	for ident, metrics := range metricList {
		for _, metric := range metrics {
			p := influxdb2.NewPointWithMeasurement("stat").
			AddTag("collector", "nfcapd").
			AddTag("version", "1.7-beta").
			AddTag("ident", ident).
			AddTag("exporterID", strconv.FormatUint(metric.exporterID, 10)).
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
			// write point asynchronously
			writeAPI.WritePoint(p)
		}
	}
	mutex.Unlock()
	// Flush writes
	writeAPI.Flush()
}

func (influxDB *influxDBConf) Loop(done chan bool) {

	tick := time.Tick(30 * time.Second)
	for {
		select {
		// Got a timeout! fail with a timeout error
		case <-done:
			fmt.Printf("Exit loop\n")
			return
		// Got a tick, we should check on doSomething()
		case <-tick:
			influxDB.insertStat()
		}
	}
} // End of Loop

// cleanup on signal TERM/cntrl-C
func SetupCloseHandler(socketHandler *socketConf, done chan bool) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		socketHandler.Close()
		os.Remove(*socketPath)
		done <- true
		fmt.Printf("Exit exporter\n")
	}()
}

func main() {

	flag.Parse()
	influxDB := NewExporter(*influxHost, *org, *bucket, *token)

	mutex = new(sync.Mutex)

	socketHandler := New(*socketPath)
	if err := socketHandler.Open(); err != nil {
		log.Fatal("Socket handler failed: ", err)
	}
	done := make(chan bool)
	SetupCloseHandler(socketHandler, done)

	socketHandler.Run()
	influxDB.Loop(done)
}
