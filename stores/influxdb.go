package stores

import (
	"fmt"
	"github.com/graphite-ng/graphite-ng/chains"
	"github.com/graphite-ng/graphite-ng/config"
	"github.com/graphite-ng/graphite-ng/metrics"
	"github.com/graphite-ng/graphite-ng/util"
	influxdb "github.com/influxdata/influxdb1-client/v2"
)

type InfluxdbStore struct {
	client influxdb.Client
	Database string
}

func NewInfluxStore(config config.Main) Store {
	c := influxdb.HTTPConfig{
		Addr:     config.StoreInflux.Addr,
		Username: config.StoreInflux.Username,
		Password: config.StoreInflux.Password,
	}
	client, err := influxdb.NewHTTPClient(c)
	util.DieIfError(err)
	return InfluxdbStore{client,config.StoreInflux.Database}
}

func init() {
	InitFn["influxdb"] = NewInfluxStore
}

func (i InfluxdbStore) Add(metric metrics.Metric) (err error) {
	panic("todo")
}

func (i InfluxdbStore) Get(name string) (our_el *chains.ChainEl, err error) {

	our_el = chains.NewChainEl()
	go func(our_el *chains.ChainEl) {
		from := <-our_el.Settings
		until := <-our_el.Settings
		queryString := fmt.Sprintf("select time, value from %s where time > %ds and time < %ds order asc", name, from, until)
		query := influxdb.NewQuery(queryString,i.Database,"")
		response, err := i.client.Query(query)
		if err != nil || response.Error() != nil {
			panic(err)
		}
		// len(series) can be 0 if there's no datapoints matching the range.
		// so it's up to the caller to make sure the store is supposed to have the data
		// if we don't have enough data to cover the requested timespan, fill with nils
		if len(response.Results) > 0 {
			series := response.Results[0].Series
			points := series[0].Values
			oldest_dp := int32(points[0][0].(float64) / 1000)
			latest_dp := int32(points[len(points)-1][0].(float64) / 1000)
			if oldest_dp > from {
				for new_ts := from; new_ts < oldest_dp; new_ts += 60 {
					our_el.Link <- *metrics.NewDatapoint(new_ts, 0.0, false)
				}
			}
			for _, values := range points {
				ts := int32(values[0].(float64) / 1000)
				val := values[2].(float64)
				dp := metrics.NewDatapoint(ts, val, true)
				our_el.Link <- *dp
			}
			if latest_dp < until {
				for new_ts := latest_dp + 60; new_ts <= until+60; new_ts += 60 {
					our_el.Link <- *metrics.NewDatapoint(new_ts, 0.0, false)
				}
			}
		} else {
			for ts := from; ts <= until+60; ts += 60 {
				our_el.Link <- *metrics.NewDatapoint(ts, 0.0, false)
			}
		}
	}(our_el)
	return our_el, nil
}

func (i InfluxdbStore) Has(name string) (found bool, err error) {
	query := influxdb.NewQuery("select time from " + name + " limit 1;",i.Database,"")
	response, err := i.client.Query(query)
	if err != nil {
		panic(err)
	}
	if len(response.Results[0].Series) > 0 {
		found = true
	}
	return
}
func (i InfluxdbStore) List() (list []string, err error) {
	query := influxdb.NewQuery("list series",i.Database,"")
	response, err := i.client.Query(query)
	if err != nil {
		return
	}
	series := response.Results[0].Series;
	list = make([]string, len(series[0].Values))
	for i, s := range series[0].Values {
		list[i] = s[0].(string)
	}
	return
}
