FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /spectrum-virtualize-exporter .

FROM alpine:3.21
RUN addgroup -g 1000 -S spectrum && adduser -u 1000 -S spectrum -G spectrum
COPY --from=builder /spectrum-virtualize-exporter /opt/spectrumVirtualize/spectrum-virtualize-exporter
RUN chown 1000:1000 /opt/spectrumVirtualize/spectrum-virtualize-exporter
USER spectrum
EXPOSE 9119
ENTRYPOINT ["/opt/spectrumVirtualize/spectrum-virtualize-exporter"]
CMD ["--config.file=/etc/spectrumVirtualize/spectrumVirtualize.yml"]
