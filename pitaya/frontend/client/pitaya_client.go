package main

import (
	"flag"
	"github.com/rpcxio/rpcx-benchmark/stat"
	"github.com/smallnest/rpcx/log"
	"github.com/topfreegames/pitaya/v2/client"
	"github.com/topfreegames/pitaya/v2/conn/message"
	"go.uber.org/ratelimit"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rpcxio/rpcx-benchmark/proto"
)

var (
	concurrency = flag.Int("c", 1000, "concurrency")
	total       = flag.Int("n", 1000000, "total requests for all clients")
	host        = flag.String("s", "127.0.0.1:7441", "server ip and port")
	pool        = flag.Int("pool", 100, "shared rpcx clients")
	rate        = flag.Int("r", 0, "throughputs")
)

func main() {
	flag.Parse()

	var rl ratelimit.Limiter
	if *rate > 0 {
		rl = ratelimit.New(*rate)
	}

	// 并发goroutine数.模拟客户端
	n := *concurrency
	// 每个客户端需要发送的请求数
	m := *total / n
	// 每个client负载的请求数
	k := *total / (*pool)
	log.Infof("concurrency: %d  requests per goruntine: %d\n", n, m)
	log.Infof("pool: %d  requests per client: %d\n", *pool, k)

	// 创建服务端的信息
	servers := strings.Split(*host, ",")
	log.Infof("Servers: %+v\n", *host)
	// 准备好参数
	args := proto.PrepareArgs()

	// 参数的大小
	b, _ := args.Marshal()
	log.Infof("message size: %d bytes\n\n", len(b))

	// 等待所有测试完成
	var wg sync.WaitGroup
	wg.Add(n * m)

	// 创建客户端连接池
	var clientIndex uint64
	poolClients := make([]*client.Client, 0, *pool)
	for i := 0; i < *pool; i++ {
		xclient := client.New(4, 100*time.Second)
		xclient.IncomingMsgChan = make(chan *message.Message, k)
		err := xclient.ConnectTo(servers[0])
		if err != nil {
			panic(err)
		}
		defer xclient.Disconnect()

		//// warmup
		//var reply proto.BenchmarkMessage
		//for j := 0; j < 5; j++ {
		//	xclient.SendRequest("connector.testsvc.say",b)
		//}

		poolClients = append(poolClients, xclient)
	}

	// 栅栏，控制客户端同时开始测试
	var startWg sync.WaitGroup
	startWg.Add(n + 1) // +1 是因为有一个goroutine用来记录开始时间

	// 总请求数
	var trans uint64
	// 返回正常的总请求数
	var transOK uint64

	// 每个goroutine的耗时记录
	d := make([][]int64, n, n)

	// 创建客户端 goroutine 并进行测试
	startTime := time.Now().UnixNano()
	go func() {
		startWg.Done()
		startWg.Wait()
		startTime = time.Now().UnixNano()
	}()

	// 每个client的开始时间记录
	cd := make([][]int64, 0, *pool)
	for i := 0; i < n; i++ {
		dt := make([]int64, k, k)
		cd = append(cd, dt)
	}

	for i := 0; i < *pool; i++ {
		xclient := poolClients[i]
		go func(i int) {
			for {
				select {
				case replyData := <-xclient.IncomingMsgChan:
					var reply proto.BenchmarkMessage
					reply.Unmarshal(replyData.Data)
					cd[i][replyData.ID-1] = time.Now().UnixNano() - cd[i][replyData.ID-1]
					if reply.Field1 == "OK" {
						atomic.AddUint64(&transOK, 1)
					} else {
						log.Errorf("invalid response")
					}
					atomic.AddUint64(&trans, 1)
					wg.Done()
				}
			}
		}(i)
	}
	for i := 0; i < n; i++ {
		dt := make([]int64, 0, m)
		d = append(d, dt)

		go func(i int) {
			//var reply proto.BenchmarkMessage

			startWg.Done()
			startWg.Wait()

			for j := 0; j < m; j++ {
				// 限流，这里不把限流的时间计算到等待耗时中
				if rl != nil {
					rl.Take()
				}

				t := time.Now().UnixNano()
				ci := atomic.AddUint64(&clientIndex, 1)
				ci = ci % uint64(*pool)
				xclient := poolClients[int(ci)]

				mid, err := xclient.SendRequest("connector.testsvc.say", b)
				if err == nil {
					cd[ci][mid-1] = t
				}
				//replyData := <-xclient.IncomingMsgChan
				//if replyData.ID != mid {
				//	log.Errorf("reqId(%v) unequal respId(%v)", mid, replyData.ID)
				//}
				//reply.Unmarshal(replyData.Data)
				//t = time.Now().UnixNano() - t // 等待时间+服务时间，等待时间是客户端调度的等待时间以及服务端读取请求、调度的时间，服务时间是请求被服务处理的实际时间
				//
				//d[i] = append(d[i], t)
				//
				//if err == nil && reply.Field1 == "OK" {
				//	atomic.AddUint64(&transOK, 1)
				//}
				//
				//atomic.AddUint64(&trans, 1)
				//wg.Done()
			}
		}(i)

	}

	// 等待测试完成
	wg.Wait()

	// 统计
	stat.Stats(startTime, *total, cd, trans, transOK)
}
