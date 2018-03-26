VERSION := 0.0.1
PKGNAME := backer
LICENSE := MIT
URL := http://github.com/nickrobison/backer
RELEASE := 1
USER := backer
DESC := Simple backup client for syncing config files to the clouds
MAINTAINER := Nick Robison <nick@nickrobison.com>
DOCKER_WDIR := /tmp/fpm
DOCKER_FPM := fpm-ubuntu
PLATFORMS := linux/amd64 linux/arm linux/arm64

test:
	go test -v ./...

build:
	go build -o 'bin/backer' .

clean:
	-rm -rf bin/
	-rm backer
	-rm *.deb
	-rm packaging/debian/usr/bin/backer

release: clean
	docker pull alanfranz/fpm-within-docker:debian-jessie
	# Build
	GOOS=linux GOARCH=amd64 go build -o packaging/debian/usr/bin/backer .
	# Package
	docker run --rm -it -v "${PWD}:${DOCKER_WDIR}" -w ${DOCKER_WDIR} --entrypoint fpm alanfranz/fpm-within-docker:debian-jessie ${DEB_OPTS} \
	--iteration ${RELEASE} \
	--architecture amd64 \
	--deb-systemd go-backer.service \
	-C packaging/debian \
	${FPM_OPTS} \
	# Remove it
	rm packaging/debian/usr/bin/backer
	# Upload it
	./upload.sh ${VERSION} ${RELEASE} amd64
	# Build
	GOOS=linux GOARCH=arm go build -o packaging/debian/usr/bin/backer .
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