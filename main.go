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

package main

import (
	"flag"
	"fmt"
	"nfinflux/influx"
	"nfinflux/nfsocket"
	"os"
)

func main() {
	var defaultHost = "http://127.0.0.1:8086"
	var defaultToken string

	// get env variables, if set
	if len(os.Getenv("INFLUXDB_URL")) > 0 {
		defaultHost = os.Getenv("INFLUXDB_URL")
	}

	if len(os.Getenv("INFLUXDB_TOKEN")) > 0 {
		defaultToken = os.Getenv("INFLUXDB_TOKEN")
	}

	var (
		influxHost   = flag.String("host", defaultHost, "Address to send metric data")
		org          = flag.String("org", "Netflow", "influxDB organisation name")
		bucket       = flag.String("bucket", "life", "influxDB bucket name")
		token        = flag.String("token", defaultToken, "influxDB token")
		socketPath   = flag.String("socket", "", "Path for nfcapd collectors to connect")
		createBucket = flag.Bool("create", false, "create bucket, if it does not exist")
		cleanBucket  = flag.Bool("delete", false, "delete existing bucket first")
		twin         = flag.Int("twin", 300, "time interval in seconds of flow file")
	)

	flag.Parse()

	influxDB, err := influx.New(*influxHost, *org, *token, *createBucket || *cleanBucket)
	if err != nil {
		fmt.Printf("Error setup influxDB at %s: %v\n", *influxHost, err)
		if *createBucket || *cleanBucket {
			fmt.Printf("Make sure your DB parameters are correct and your token is authorized to create/delete buckets\n")
		}
		os.Exit(255)
	}
	if _, err := influxDB.VerifyBucket(*bucket, *createBucket, *cleanBucket); err != nil {
		fmt.Printf("Failed to veryfy bucket '%s': %v\n", *bucket, err)
		influxDB.Close()
		os.Exit(255)
	}

	if len(*socketPath) > 0 {
		nfsocket.SetupSocketFeeder(socketPath, *bucket, influxDB)
	} else {
		scanDirs := flag.Args()
		setupFileFeeder(scanDirs, *bucket, *twin, influxDB)
	}

	influxDB.Close()
}
