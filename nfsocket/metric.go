/*
 *  Copyright (c) 2022, Peter Haag
 *  All rights reserved.
 *
 *  Redistribution and use in source and binary forms, with or without
 *  modification, are permitted provided that the following conditions are met:
 *
 *   * Redistributions of source code must retain the above copyright notice,
 *	 this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above copyright notice,
 *	 this list of conditions and the following disclaimer in the documentation
 *	 and/or other materials provided with the distribution.
 *   * Neither the name of the author nor the names of its contributors may be
 *	 used to endorse or promote products derived from this software without
 *	 specific prior written permission.
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
 *
 */

/*
 * dataSocket implements a UNIX socket server to receive data from nfcapd
 * Up to now the exporter implements flows/packets/bytes counters per
 * protocol(tcp/udp/icmp/other and the source identifier from the collector
 *
 */

package nfsocket

/*

#include <stdint.h>

typedef struct message_header_s {
    char prefix;
    uint8_t version;
    uint16_t size;
    uint16_t numMetrics;
    uint16_t interval;
    uint64_t timeStamp;
    uint64_t uptime;
    char ident[128];
} message_header_t;

typedef struct metric_record_s {
	// Ident
	uint64_t	exporterID; // 32bit: exporter_id:16 engineType:8 engineID:*

	// flow stat
	uint64_t numflows_tcp;
	uint64_t numflows_udp;
	uint64_t numflows_icmp;
	uint64_t numflows_other;
	// bytes stat
	uint64_t numbytes_tcp;
	uint64_t numbytes_udp;
	uint64_t numbytes_icmp;
	uint64_t numbytes_other;
	// packet stat
	uint64_t numpackets_tcp;
	uint64_t numpackets_udp;
	uint64_t numpackets_icmp;
	uint64_t numpackets_other;
} metric_record_t;

const int header_size = sizeof(message_header_t);
const int record_size = sizeof(metric_record_t);
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"
	"unsafe"
)

const packetPrefix byte = '@'
const packetVersion byte = 1

var headerSize int = int(C.header_size)
var metricSize int = int(C.record_size)

type SocketConf struct {
	socketPath string
	listener   net.Listener
	metricChan chan metricInfo
}

func New(socketPath string, metricChan chan metricInfo) *SocketConf {
	conf := new(SocketConf)
	conf.socketPath = socketPath
	conf.metricChan = metricChan
	return conf
}

func (conf *SocketConf) Open() error {

	if err := os.RemoveAll(conf.socketPath); err != nil {
		return err
	}
	listener, err := net.Listen("unix", conf.socketPath)
	if err != nil {
		return err
	}
	conf.listener = listener
	return nil

} // End of Open

func (conf *SocketConf) Close() error {

	err := conf.listener.Close()
	os.Remove(conf.socketPath)
	return err

} // End of Close

func processStat(conf *SocketConf, conn net.Conn) {

	defer conn.Close()

	// storage for reading from socket.
	readBuf := make([]byte, 65536)

	dataLen, err := conn.Read(readBuf)
	if err != nil || dataLen == 0 {
		fmt.Printf("Socket read error: %v\n", err)
		return
	}

	// message prefix
	if readBuf[0] != packetPrefix {
		fmt.Printf("Message prefix error - got %d\n", readBuf[0])
		return
	}

	// version
	if readBuf[1] != packetVersion {
		fmt.Printf("Message prefix error - unknow version %d\n", readBuf[1])
		return
	}

	payloadSize := int(binary.LittleEndian.Uint16(readBuf[2:4]))
	if dataLen < payloadSize {
		fmt.Printf("Message size error - received %d, announced %d\n", dataLen, payloadSize)
		return
	}

	numMetrics := int(binary.LittleEndian.Uint16(readBuf[4:6]))
	ilen := 0
	for i := 0; readBuf[24+i] != 0; i++ {
		ilen++
	}
	ident := string(readBuf[24 : 24+ilen])
	timestamp := int(binary.LittleEndian.Uint64(readBuf[8:16]))

	/*
		interval	:= int(binary.LittleEndian.Uint64(readBuf[6:8]))
		timestamp	:= int(binary.LittleEndian.Uint64(readBuf[8:16]))
		uptime		:= int(binary.LittleEndian.Uint64(readBuf[16:24]))
		fmt.Printf("Message size: %d, payload size: %d version: %d, numMetrics: %d\n",
			dataLen, payloadSize, version, numMetrics);
		fmt.Printf("Time(ms): %d, interval: %d, uptime: %d, ident: %s\n",
			timestamp, interval, uptime, ident)
	*/

	offset := headerSize
	for num := 0; num < numMetrics; num++ {
		availableSize := dataLen - offset
		if availableSize < metricSize {
			fmt.Printf("Message size error - left %d, expected %d\n", availableSize, metricSize)
			return
		}
		var s *C.metric_record_t = (*C.metric_record_t)(unsafe.Pointer(&readBuf[offset]))
		metric := metricInfo{}
		metric.exporter = int(s.exporterID)
		metric.timestamp = uint64(timestamp)
		metric.ident = ident

		metric.stat.NumflowsTcp = uint64(s.numflows_tcp)
		metric.stat.NumflowsUdp = uint64(s.numflows_udp)
		metric.stat.NumflowsIcmp = uint64(s.numflows_icmp)
		metric.stat.NumflowsOther = uint64(s.numflows_other)

		metric.stat.NumbytesTcp = uint64(s.numbytes_tcp)
		metric.stat.NumbytesUdp = uint64(s.numbytes_udp)
		metric.stat.NumbytesIcmp = uint64(s.numbytes_icmp)
		metric.stat.NumbytesOther = uint64(s.numbytes_other)

		metric.stat.NumpacketsTcp = uint64(s.numpackets_tcp)
		metric.stat.NumpacketsUdp = uint64(s.numpackets_udp)
		metric.stat.NumpacketsIcmp = uint64(s.numpackets_icmp)
		metric.stat.NumpacketsOther = uint64(s.numpackets_other)

		fmt.Printf("Received metric for '%s', exporter: %d, at %v\n", ident, metric.exporter, time.UnixMilli(int64(timestamp)))
		conf.metricChan <- metric
		offset += metricSize
	}

} // end of processStat

func (conf *SocketConf) Run(done chan bool) {

	go func() {
		for {
			// Accept new connections from nfcapd collectors and
			// dispatching them to goroutine processStat
			conn, err := conf.listener.Accept()
			if err == nil {
				go processStat(conf, conn)
			} else {
				select {
				case <-done:
					close(conf.metricChan)
					return
				default:
					fmt.Printf("Accept() error: %v\n", err)
				}
			}
		}
	}()

} // End of Run
