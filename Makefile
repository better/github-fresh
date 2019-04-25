NAME=github-fresh
VERSION=0.5.0
COMMIT=$(shell git rev-parse --short=7 HEAD)
TIMESTAMP:=$(shell date -u '+%Y-%m-%dT%I:%M:%SZ')

LDFLAGS += -X main.BuildTime=${TIMESTAMP}
LDFLAGS += -X main.BuildSHA=${COMMIT}
LDFLAGS += -X main.Version=${VERSION}

DOCKER=$(shell command -v docker;)

.PHONY: all
all: quality test build
ifneq (${DOCKER},)
	${MAKE} docker
else
	:
endif

.PHONY: quality
quality:
	go vet
	go fmt
ifneq (${DOCKER},)
	docker run -v ${PWD}:/src -w /src -it golangci/golangci-lint golangci-lint run --enable gocritic --enable gosec --enable golint --enable stylecheck --exclude-use-default=false
endif

.PHONY: test
test:
	go test -race -coverprofile=coverage

.PHONY: clean
clean:
	rm -f ${NAME}*

.PHONY: build
build: clean build-darwin build-linux

build-%:
	GOOS=$* GOARCH=386 go build -ldflags '${LDFLAGS}' -o ${NAME}-$*

.PHONY: docker
docker:
	docker run --rm -i hadolint/hadolint < Dockerfile
	docker build \
	--build-arg NAME="${NAME}" \
	--build-arg VERSION="${VERSION}" \
	--build-arg COMMIT="${COMMIT}" \
	--build-arg BUILD_DATE="${TIMESTAMP}" \
	--build-arg LDFLAGS='${LDFLAGS}' \
	--tag ${NAME} .
	docker tag ${NAME} ${NAME}:${VERSION}
	docker run -it ${NAME}:${VERSION} -- -help 2>&1 | grep -F '${NAME} v${VERSION} ${TIMESTAMP} ${COMMIT}'
