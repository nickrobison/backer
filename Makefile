VERSION_RELEASE = $(subst -, ,$(shell cat VERSION))
VERSION := $(word 1, $(VERSION_RELEASE))
PKGNAME := backer
LICENSE := MIT
URL := http://github.com/nickrobison/backer
RELEASE := $(word 2, $(VERSION_RELEASE))
USER := backer
DESC := Simple backup client for syncing config files to the clouds
MAINTAINER := Nick Robison <nick@nickrobison.com>
DOCKER_WDIR := /tmp/fpm
DOCKER_FPM := fpm-ubuntu
PLATFORMS := linux/amd64 linux/arm linux/arm64
LD_FLAGS := "-X main.Version=$(VERSION)"

FPM_OPTS :=-s dir -v $(VERSION) -n $(PKGNAME) \
  --license "$(LICENSE)" \
  --vendor "$(VENDOR)" \
  --maintainer "$(MAINTAINER)" \
  --url "$(URL)" \
  --description  "$(DESC)" \
  --verbose

DEB_OPTS := -t deb --deb-user $(USER) --after-install packaging/debian/backer.postinst

test:
	go test -race -v ./...

build:
	go build -ldflags $(LD_FLAGS) -o 'bin/backer' .

clean:
	-rm -rf bin/
	-rm backer
	-rm *.deb
	-rm packaging/debian/usr/bin/backer

release: clean
	docker pull alanfranz/fpm-within-docker:debian-jessie
	# Build
	GOOS=linux GOARCH=amd64 go build -ldflags $(LD_FLAGS) -o packaging/debian/usr/bin/backer .
	# Package
	docker run --rm -it -v "${PWD}:${DOCKER_WDIR}" -w ${DOCKER_WDIR} --entrypoint fpm alanfranz/fpm-within-docker:debian-jessie ${DEB_OPTS} \
	--iteration ${RELEASE} \
	--architecture amd64 \
	--deb-systemd backer.service \
	-C packaging/debian \
	${FPM_OPTS} \
	# Remove it
	rm packaging/debian/usr/bin/backer
	# Upload it
	./upload.sh ${VERSION} ${RELEASE} amd64
	# Build
	GOOS=linux GOARCH=arm go build -ldflags $(LD_FLAGS) -o packaging/debian/usr/bin/backer .
	# Package
	docker run --rm -it -v "${PWD}:${DOCKER_WDIR}" -w ${DOCKER_WDIR} --entrypoint fpm alanfranz/fpm-within-docker:debian-jessie ${DEB_OPTS} \
	--iteration ${RELEASE} \
	--architecture armhf \
	--deb-systemd backer.service \
	-C packaging/debian \
	${FPM_OPTS} \
	# Remove it
	rm packaging/debian/usr/bin/backer
	# Upload everything
	./upload.sh ${VERSION} ${RELEASE} armhf



.PHONY: test build release clean