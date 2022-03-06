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

package nfsocket

import (
	"fmt"
	"log"
	"nfinflux/influx"
	"nfinflux/nffile"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type metricInfo struct {
	timestamp uint64
	ident     string
	exporter  int
	stat      nffile.StatRecord
}

// feed data to influx DB ever 60s
func runFeeder(influxDB *influx.InfluxDBConf, bucket string, metricChan chan metricInfo) {

	influxDB.StartWrite(bucket)
	for metricRecord := range metricChan {
		influxDB.InsertStat(time.UnixMilli(int64(metricRecord.timestamp)), metricRecord.ident, strconv.Itoa(metricRecord.exporter), metricRecord.stat)
		fmt.Printf("Insert stat for '%s', exporter: %d, at %v\n", metricRecord.ident, metricRecord.exporter, time.UnixMilli(int64(metricRecord.timestamp)))
	}
	influxDB.EndWrite()
	fmt.Printf("Exit influx feeder\n")
} // End of runFeeder

// wait for signal TERM/INT(cntrl-C) and close done chan
func SetupCloseHandler(socketHandler *SocketConf) chan bool {
	done := make(chan bool)
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("nfinflux interrupted\n")
		close(done)
		socketHandler.Close()
	}()
	return done
}

// Listen on feeder socket and call feeder loop
func SetupSocketFeeder(socketPath *string, bucket string, influxDB *influx.InfluxDBConf) {

	// received data goes into the metric list
	metricChan := make(chan metricInfo, 128)
	socketHandler := New(*socketPath, metricChan)
	if err := socketHandler.Open(); err != nil {
		log.Fatal("Socket handler failed: ", err)
	}

	fmt.Printf("nfinflux ready to accept connections\n")

	// returns channel getting closed on signal
	done := SetupCloseHandler(socketHandler)
	// accepts connections until done closed
	socketHandler.Run(done)
	// metricChan gets closed by socketHandler in case of signal
	runFeeder(influxDB, bucket, metricChan)
	fmt.Printf("nfinflux terminated\n")
}
