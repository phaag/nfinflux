/*
 *  Copyright (c) 2022, Peter Haag
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

package influx

import (
	"context"
	"fmt"
	"nfinflux/nffile"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

type InfluxDBConf struct {
	host     string
	token    string
	org      string
	client   influxdb2.Client
	writeAPI api.WriteAPI
	errList  []error
	dOrg     *domain.Organization
}

func New(host string, org string, token string, verifyOrg bool) (*InfluxDBConf, error) {
	influxDB := new(InfluxDBConf)
	influxDB.host = host
	influxDB.token = token
	influxDB.org = org

	client := influxdb2.NewClientWithOptions(host, token,
		influxdb2.DefaultOptions().SetPrecision(time.Millisecond))

	influxDB.client = client

	if verifyOrg {
		ctx := context.Background()
		if dOrg, err := client.OrganizationsAPI().FindOrganizationByName(ctx, org); err != nil {
			return nil, err
		} else {
			influxDB.dOrg = dOrg
		}
	}
	return influxDB, nil
} // End of NewExporter

func (influxDB *InfluxDBConf) Close() error {
	if influxDB.client != nil {
		influxDB.client.Close()
	}
	return nil
}

func (influxDB *InfluxDBConf) VerifyBucket(bucket string, createMissing bool, deleteExisting bool) (*domain.Bucket, error) {
	bucketsAPI := influxDB.client.BucketsAPI()

	var dBucket *domain.Bucket
	var err error
	var ctx context.Context

	// delete bucket if reqested
	if deleteExisting {
		ctx = context.Background()
		dBucket, err = bucketsAPI.FindBucketByName(ctx, bucket)
		if err == nil {
			// bucket exists - delete it
			ctx = context.Background()
			if err = bucketsAPI.DeleteBucket(ctx, dBucket); err != nil {
				return nil, err
			}
		}
		dBucket = nil
		createMissing = true
	}

	// get bucket
	ctx = context.Background()
	dBucket, err = bucketsAPI.FindBucketByName(ctx, bucket)

	// create bucket if requested
	if err != nil && createMissing {
		// Create  a bucket with x day retention policy
		ctx = context.Background()
		if dBucket, err = bucketsAPI.CreateBucketWithName(ctx, influxDB.dOrg, bucket, domain.RetentionRule{EverySeconds: 0}); err != nil {
			return dBucket, err
		}

		// Update description of the bucket
		desc := "Flow data bucket"
		dBucket.Description = &desc
		if dBucket, err = bucketsAPI.UpdateBucket(ctx, dBucket); err != nil {
			return dBucket, err
		}
	}

	return dBucket, err
}

func (influxDB *InfluxDBConf) StartWrite(bucket string) {
	// get non-blocking write client
	writeAPI := influxDB.client.WriteAPI(influxDB.org, bucket)
	// Get errors channel
	errorsCh := writeAPI.Errors()
	// Create go proc for reading and logging errors
	go func() {
		for err := range errorsCh {
			influxDB.errList = append(influxDB.errList, err)
			fmt.Printf("write error: %s\n", err.Error())
		}
	}()
	influxDB.writeAPI = writeAPI
}

func (influxDB *InfluxDBConf) Flush() {
	// Flush writes
	influxDB.writeAPI.Flush()
}

func (influxDB *InfluxDBConf) EndWrite() error {
	// Flush writes
	influxDB.writeAPI.Flush()
	influxDB.writeAPI = nil
	numErrors := 0
	if influxDB.errList != nil {
		numErrors = len(influxDB.errList)
		return fmt.Errorf("failed writes: %d", numErrors)
	}
	return nil
}

func (influxDB *InfluxDBConf) InsertStat(when time.Time, ident string, exporterID string, statRecord nffile.StatRecord) {
	if influxDB.writeAPI == nil {
		return
	}

	writeAPI := influxDB.writeAPI

	// create point proto tcp
	p := write.NewPoint(
		"stat",
		map[string]string{
			"channel": ident,
			// "exporter": exporterID,
			"proto": "tcp",
		},
		map[string]interface{}{
			"flows":   statRecord.NumflowsTcp,
			"packets": statRecord.NumpacketsTcp,
			"bytes":   statRecord.NumbytesTcp,
		},
		when)
	writeAPI.WritePoint(p)

	p = write.NewPoint(
		"stat",
		map[string]string{
			"channel": ident,
			// "exporter": exporterID,
			"proto": "udp",
		},
		map[string]interface{}{
			"flows":   statRecord.NumflowsUdp,
			"packets": statRecord.NumpacketsUdp,
			"bytes":   statRecord.NumbytesUdp,
		},
		when)
	writeAPI.WritePoint(p)

	p = write.NewPoint(
		"stat",
		map[string]string{
			"channel": ident,
			//		"exporter": exporterID,
			"proto": "icmp",
		},
		map[string]interface{}{
			"flows":   statRecord.NumflowsIcmp,
			"packets": statRecord.NumpacketsIcmp,
			"bytes":   statRecord.NumbytesIcmp,
		},
		when)
	writeAPI.WritePoint(p)

	p = write.NewPoint(
		"stat",
		map[string]string{
			"channel": ident,
			// "exporter": exporterID,
			"proto": "other",
		},
		map[string]interface{}{
			"flows":   statRecord.NumflowsOther,
			"packets": statRecord.NumpacketsOther,
			"bytes":   statRecord.NumbytesOther,
		},
		when)
	writeAPI.WritePoint(p)

}
