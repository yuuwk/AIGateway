FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' appuser

WORKDIR /app
COPY aigateway /app/aigateway

USER appuser
EXPOSE 8080

ENTRYPOINT ["./aigateway"]
CMD ["-config", "config.yaml"]
