package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rpcxio/rpcx-benchmark/proto"
	"github.com/spf13/viper"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/acceptor"
	"github.com/topfreegames/pitaya/v2/cluster"
	"github.com/topfreegames/pitaya/v2/component"
	"github.com/topfreegames/pitaya/v2/config"
	"github.com/topfreegames/pitaya/v2/constants"
	"github.com/topfreegames/pitaya/v2/groups"
	logruswrapper "github.com/topfreegames/pitaya/v2/logger/logrus"
	"github.com/topfreegames/pitaya/v2/modules"
	"github.com/topfreegames/pitaya/v2/serialize/json"
	"github.com/topfreegames/pitaya/v2/serialize/protobuf"
	"github.com/topfreegames/pitaya/v2/session"
)

// TestSvc service for e2e tests
type TestSvc struct {
	component.Base
	app         pitaya.Pitaya
	sessionPool session.SessionPool
}

// TestRemoteSvc remote service for e2e tests
type TestRemoteSvc struct {
	component.Base
}

func (t *TestSvc) Say(ctx context.Context, args *proto.BenchmarkMessage) (*proto.BenchmarkMessage, error) {
	args.Field1 = "OK"
	args.Field2 = 100
	if *delay > 0 {
		time.Sleep(*delay)
	} else {
		runtime.Gosched()
	}
	return args, nil
}

var (
	host      = flag.String("s", "127.0.0.1:7441", "listened ip and port")
	delay     = flag.Duration("delay", 0, "delay to mock business processing")
	debugAddr = flag.String("d", "127.0.0.1:9981", "server ip and port")
)

func main() {
	isFrontend := flag.Bool("frontend", true, "if server is frontend")
	svType := flag.String("type", "connector", "the server type")
	debug := flag.Bool("debug", false, "turn on debug logging")
	grpc := flag.Bool("grpc", true, "turn on grpc")
	grpcPort := flag.Int("grpcport", 3434, "the grpc server port")
	flag.Parse()
	cfg := viper.New()
	cfg.Set("pitaya.cluster.rpc.server.grpc.port", *grpcPort)
	cfg.Set("pitaya.handler.messages.compression", false)

	l := logrus.New()
	l.Formatter = &logrus.TextFormatter{}
	l.SetLevel(logrus.WarnLevel)
	if *debug {
		l.SetLevel(logrus.DebugLevel)
	}
	pitaya.SetLogger(logruswrapper.NewWithFieldLogger(l))

	go func() {
		log.Println(http.ListenAndServe(*debugAddr, nil))
	}()

	port, _ := strconv.Atoi(strings.Split(*host, ":")[1])

	app, bs, sessionPool := createApp("protobuf", port, *grpc, *isFrontend, *svType, pitaya.Cluster, map[string]string{
		constants.GRPCHostKey: "127.0.0.1",
		constants.GRPCPortKey: fmt.Sprintf("%d", *grpcPort),
	}, cfg)

	if *grpc {
		app.RegisterModule(bs, "bindingsStorage")
	}

	app.Register(
		&TestSvc{
			app:         app,
			sessionPool: sessionPool,
		},
		component.WithName("testsvc"),
		component.WithNameFunc(strings.ToLower),
	)

	app.RegisterRemote(
		&TestRemoteSvc{},
		component.WithName("testremotesvc"),
		component.WithNameFunc(strings.ToLower),
	)

	app.Start()
}

func createApp(serializer string, port int, grpc bool, isFrontend bool, svType string, serverMode pitaya.ServerMode, metadata map[string]string, cfg ...*viper.Viper) (pitaya.Pitaya, *modules.ETCDBindingStorage, session.SessionPool) {
	conf := config.NewConfig(cfg...)
	builder := pitaya.NewBuilderWithConfigs(isFrontend, svType, serverMode, metadata, conf)

	if isFrontend {
		tcp := acceptor.NewTCPAcceptor(fmt.Sprintf(":%d", port))
		builder.AddAcceptor(tcp)
	}

	builder.Groups = groups.NewMemoryGroupService(*config.NewDefaultMemoryGroupConfig())

	if serializer == "json" {
		builder.Serializer = json.NewSerializer()
	} else if serializer == "protobuf" {
		builder.Serializer = protobuf.NewSerializer()
	} else {
		panic("serializer should be either json or protobuf")
	}

	var bs *modules.ETCDBindingStorage
	if grpc {
		gs, err := cluster.NewGRPCServer(*config.NewGRPCServerConfig(conf), builder.Server, builder.MetricsReporters)
		if err != nil {
			panic(err)
		}

		bs = modules.NewETCDBindingStorage(builder.Server, builder.SessionPool, *config.NewETCDBindingConfig(conf))

		gc, err := cluster.NewGRPCClient(
			*config.NewGRPCClientConfig(conf),
			builder.Server,
			builder.MetricsReporters,
			bs,
			cluster.NewInfoRetriever(*config.NewInfoRetrieverConfig(conf)),
		)
		if err != nil {
			panic(err)
		}
		builder.RPCServer = gs
		builder.RPCClient = gc
	}

	return builder.Build(), bs, builder.SessionPool
}
