FROM golang:1.14
 
LABEL maintainer="Rishabh Mishra<rishabhmishra131@gmail.com>"
 
WORKDIR /app
COPY test ./bin/smtplistener
COPY conf.json conf.json
EXPOSE 2525
RUN touch /tmp/test.com.key
RUN touch /tmp/test.com.crt
RUN chgrp -R 0 /run && chmod -R g=u /run
 
CMD ["./bin/smtplistener", "serve"]
