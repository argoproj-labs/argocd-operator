FROM docker.io/library/golang:1.22 AS builder

WORKDIR /app
COPY . .
RUN go build -o bin/smtplistener ./cmd/listener

FROM docker.io/library/debian:buster-slim
 
LABEL maintainer="Rishabh Mishra<rishabhmishra131@gmail.com>"
 
WORKDIR /app
COPY --from=builder /app/bin/smtplistener ./bin/smtplistener
COPY conf.json conf.json
EXPOSE 2525
RUN touch /tmp/test.com.key
RUN touch /tmp/test.com.crt
RUN chgrp -R 0 /run && chmod -R g=u /run
 
CMD ["./bin/smtplistener", "serve"]
