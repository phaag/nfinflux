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
	"fmt"
	"nfinflux/influx"
	"nfinflux/nffile"
	"os"
	"path/filepath"
	"time"
)

type flowFile struct {
	fileName string
	timeSlot time.Time
}

// recursively iterate over all files and directories and push all entries in FileQ
func enumerateFiles(pathList []string) chan flowFile {

	fileChannel := make(chan flowFile, 16)

	go func() {
		for _, path := range pathList {
			filepath.Walk(path, func(entry string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					return nil
				}
				// check if it is a regular file
				if info.Mode().IsRegular() {
					var timeString string
					fileName := info.Name()
					// check if it's a know nfcapd.xx file name
					if n, _ := fmt.Sscanf(fileName, "nfcapd.%s", &timeString); n != 0 {
						var t time.Time
						t, err = time.Parse("200601021504", timeString)
						if err == nil {
							fileChannel <- flowFile{entry, t}
							// fmt.Printf("file path: %s Time: %v, %v\n", entry, t, t.UnixMilli())
						} else {
							fmt.Printf("Error parsing time: %s\n", timeString)
						}
					}
				}
				return nil
			})
		}
		close(fileChannel)
	}()

	return fileChannel
} // End of enumerateFiles

// create the rate e.g. values/s for fps, pps and bps
func calculateRate(stat *nffile.StatRecord, rate uint64) {
	stat.Numflows /= rate
	stat.Numbytes /= rate
	stat.Numpackets /= rate
	// flow stat /= rate
	stat.NumflowsTcp /= rate
	stat.NumflowsUdp /= rate
	stat.NumflowsIcmp /= rate
	stat.NumflowsOther /= rate
	// bytes stat
	stat.NumbytesTcp /= rate
	stat.NumbytesUdp /= rate
	stat.NumbytesIcmp /= rate
	stat.NumbytesOther /= rate
	// packet stat
	stat.NumpacketsTcp /= rate
	stat.NumpacketsUdp /= rate
	stat.NumpacketsIcmp /= rate
	stat.NumpacketsOther /= rate
}

func setupFileFeeder(scanDirs []string, bucket string, twin int, influxDB *influx.InfluxDBConf) {
	fileChannel := enumerateFiles((scanDirs))

	nfFile := nffile.New()
	influxDB.StartWrite(bucket)
	exporterID := "0"
	fileCnt := 0
	for file := range fileChannel {
		fmt.Printf("file path: %s Time: %v\r", file.fileName, file.timeSlot)
		if err := nfFile.Open(file.fileName); err != nil {
			panic(err)
		}
		// nfFile.String()
		stat := nfFile.Stat()
		calculateRate(&stat, uint64(twin))
		influxDB.InsertStat(file.timeSlot, nfFile.Ident(), exporterID, stat)
		nfFile.Close()
		fileCnt++
	}
	fmt.Printf("\nInsert stat, processed %d files\n", fileCnt)
	if err := influxDB.EndWrite(); err != nil {
		fmt.Printf("Insert stat record(s): %v\n", err)
	}
}
