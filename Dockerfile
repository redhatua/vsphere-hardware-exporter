# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/vsphere-hardware-exporter ./cmd/vsphere-hardware-exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/vsphere-hardware-exporter /vsphere-hardware-exporter
EXPOSE 9877
USER nonroot:nonroot
ENTRYPOINT ["/vsphere-hardware-exporter"]
