FROM alpine

ADD dist/go-url-shortener /bin/
ADD config.json /config.json

EXPOSE 8080

ENTRYPOINT [ "/bin/go-url-shortener" ]
