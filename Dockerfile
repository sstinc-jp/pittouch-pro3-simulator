FROM golang:1.23-bookworm

WORKDIR /usr/app

# goのファイルをコピー
COPY prooperate/ ./prooperate
COPY websql/ ./websql
COPY go.mod ./
COPY go.sum ./
COPY main.go ./

# コンテナ内でも使えるようにtoolsをコピー
COPY tools ./tools

RUN pwd
RUN go build -o pro3sim


#CMD [ "./pro3sim", "-ctsDir=./cts" ]