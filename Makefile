TARGETDIR='TARGETDIR is undefined'
CLIENTS=1000
REQUEST_NUM=1000000
LISTEN=0.0.0.0:7441
ADDR=127.0.0.1:7441


build-one:
	@mkdir -p bin/$(TARGETDIR)/$(SUBDIR)
	@cd $(TARGETDIR) && go build -o ../bin/$(TARGETDIR)/$(SUBDIR)server ./$(SUBDIR)server
	@cd $(TARGETDIR) && go build -o ../bin/$(TARGETDIR)/$(SUBDIR)client ./$(SUBDIR)client

clean-one:
	@rm -rf bin/$(TARGETDIR)

rebuild-one:clean-one build-one

#启动测试需要的依赖设施
ensure-deps-one:
	@cd $(TARGETDIR) && docker-compose up -d
#停止测试需要的依赖设施
kill-deps-one:
	@cd $(TARGETDIR) && docker-compose down; true
restart-deps-one:kill-deps-one ensure-deps-one

#test-one:build-one
#	@cd bin/$(TARGETDIR) && ./server -s $(LISTEN)
#	@cd bin/$(TARGETDIR) && ./client -s $(ADDR) -c $(CLIENTS) -n $(REQUEST_NUM)


clean-rpcx:
	@$(MAKE) clean-one TARGETDIR=rpcx
build-rpcx:
	@$(MAKE) build-one TARGETDIR=rpcx

clean-grpc:
	@$(MAKE) clean-one TARGETDIR=grpc
build-grpc:
	@$(MAKE) build-one TARGETDIR=grpc

clean-kitex:
	@$(MAKE) clean-one TARGETDIR=kitex
build-kitex:
	@$(MAKE) build-one TARGETDIR=kitex

clean-pitaya:
	@$(MAKE) clean-one TARGETDIR=pitaya
build-pitaya-frontend:
	@$(MAKE) build-one TARGETDIR=pitaya SUBDIR=frontend/
build-pitaya-nats:
	@$(MAKE) build-one TARGETDIR=pitaya SUBDIR=nats/
ensure-deps-pitaya:
	@$(MAKE) ensure-deps-one TARGETDIR=pitaya
kill-deps-pitaya:
	@$(MAKE) kill-deps-one TARGETDIR=pitaya
restart-deps-pitaya:
	@$(MAKE) restart-deps-one TARGETDIR=pitaya
#
#test-rpcx:
#	@$(MAKE) test-one TARGETDIR=rpcx

