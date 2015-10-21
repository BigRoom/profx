FROM golang:1.5.1

MAINTAINER Harrison Shoebridge <harrison@theshoebridges.com>

RUN go get github.com/bigroom/vision/tunnel
RUN go get github.com/nickvanw/ircx
RUN go get github.com/paked/configure
RUN go get github.com/sorcix/irc

ADD . /go/src/github.com/bigroom/roomer


WORKDIR /go/src/github.com/bigroom/roomer
CMD go run bot.go
