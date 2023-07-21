BUILDDIR=$(shell pwd)/libclash.xcframework
BuildTime=$(shell date "+%Y-%m-%d-%H%M")
Version="5212aa"

all: ios

ios: clean
	mkdir -p $(BUILDDIR)
	gomobile bind -o ${BUILDDIR} -a -ldflags "-X \"github.com/Dreamacro/clash/constant.Version=${Version}\" -X \"github.com/Dreamacro/clash/constant.BuildTime=${BuildTime}\"" -target=ios .


clean:
	gomobile clean
	rm -rf $(BUILDDIR)

cleanmodcache:
	go clean -modcache
