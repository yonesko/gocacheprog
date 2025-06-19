FROM repo.int.tsum.com/tsum/core/golang:1.24.1
LABEL authors="gdanichev"
WORKDIR /app
COPY . .
RUN go build -o /usr/local/go/bin/gocacheprog .

ENTRYPOINT ["bash"]
