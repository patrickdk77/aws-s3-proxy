BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
SHA1 := $(shell git rev-parse HEAD)
SHORT_SHA1 := $(shell git rev-parse --short HEAD)
ORIGIN := $(shell git remote get-url origin)
DATE := $(date -u +'%Y-%m-%dT%H:%M:%Sz')
VER := $(shell git describe --tags --abbrev=0)
DOCK_REPO := patrickdk/s3proxy

export DOCKERFILE_PATH=Dockerfile
export DOCKER_REPO=$(DOCK_REPO)
export DOCKER_TAG=latest
export GIT_BRANCH=$(BRANCH)
export GIT_SHA1=$(SHA1)
export GIT_TAG=$(SHA1)
export GIT_VERSION=$(VER)
export IMAGE_NAME=$(DOCKER_REPO):$(DOCKER_TAG)
export SOURCE_BRANCH=$(BRANCH)
export SOURCE_COMMIT=$(SHA1)
export SOURCE_TYPE=git
export SOURCE_REPOSITORY_URL=$(ORIGIN)
export DOCKER_EXTRATAGS=latest

all: build

build: export DOCKER_TAG=$(GIT_VERSION)
build: docker

docker:
	./hooks/post_checkout
	./hooks/pre_build
	./hooks/build
#	./hooks/push

release:
	./hooks/push

deps:
	@docker run --rm -it -v "${PWD}:/go/src/github.com/patrickdk77/aws-s3-proxy/" \
			-w /go/src/github.com/patrickdk77/aws-s3-proxy/ \
			golang:1.16-alpine3.12 sh -c 'apk --no-cache add git && go mod vendor'

up:
	@docker-compose up -d

logs:
	@docker-compose logs -f

down:
	@docker-compose down -v

test:
	@docker run --rm -it -v "${PWD}:/go/src/github.com/patrickdk77/aws-s3-proxy/" \
			-w /go/src/github.com/patrickdk77/aws-s3-proxy/ \
            golangci/golangci-lint:v1.36-alpine \
			golangci-lint run --config .golangci.yml
	@docker run --rm -it -v "${PWD}:/go/src/github.com/patrickdk77/aws-s3-proxy/" \
			-w /go/src/github.com/patrickdk77/aws-s3-proxy/ \
			golangci/golangci-lint:v1.36-alpine \
			sh -c "go list ./... | grep -v /vendor/ | xargs go test -p 1 -count=1"

#build:
#	@docker run --rm -it -v "${PWD}:/go/src/github.com/patrickdk77/aws-s3-proxy/" \
#			-w /go/src/github.com/patrickdk77/aws-s3-proxy/ \
#			supinf/go-gox:1.11 --osarch "linux/amd64" \
#			-ldflags "-s -w" -output "dist/{{.OS}}_{{.Arch}}"


