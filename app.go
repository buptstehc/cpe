package main

import (
	"fmt"
	"flag"
	"time"
	"cpe/util"
	"os"
	"cpe/plugin"
	"strconv"
	"net/http"
	"encoding/json"
	_ "net/http/pprof"
)

func main() {
	c := flag.String("c", "config.json", "config file path")
	flag.Parse()

	//解析配置文件
	config, err := util.ReadConfig(*c)
	if !err {
		os.Exit(1)
	}
	totalIp := len(config.IP)
	if totalIp == 0 {
		fmt.Println("Error: Local IP must be set")
		os.Exit(1)
	}

	//collect metric
	ch := make(chan plugin.Msg, 1024)
	metrics := plugin.Metric{}
	go func() {
		for {
			msg := <-ch
			switch msg.Event {
			case plugin.E_ABORT:
				metrics.Aborts++
			case plugin.E_BOOT_SENT:
				metrics.BootSent++
			case plugin.E_BOOT_REPLY:
				metrics.BootReply++
			case plugin.E_REG_SENT:
				metrics.RegSent++
			case plugin.E_REG_REPLY:
				metrics.RegReply++
			case plugin.E_PING:
				metrics.Pings++
			case plugin.E_PONG:
				metrics.Pongs++
			}
		}
	}()

	//start cpe
	go func() {
		n := 0
		for i := 0; i < config.Cpe; i++ {
			if i != 0 {
				if i%config.Qps == 0 {
					time.Sleep(1 * time.Second)
				}

				if i%config.ClientsPerIP == 0 {
					n++
					if n == totalIp {
						fmt.Println("Error: No more local IP...")
						break
					}
				}
			}

			prefix := strconv.Itoa(i + 1)
			client := plugin.NewTCPConn(ch, uint32(i+1), config.IP[n], config.Vertx,
				config.Mac+prefix, config.SN+prefix, config.LOID+prefix)

			go client.Run()
		}
	}()

	http.HandleFunc("/m", func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(metrics)
		fmt.Fprintf(w, string(b))
	})
	http.ListenAndServe(":"+strconv.Itoa(config.MetricPort), nil)
}
