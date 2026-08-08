FROM golang:1.26.5-bookworm AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . ./

ARG VERSION
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/garfex ./cmd/garfex

FROM gcr.io/distroless/static-debian12:nonroot

COPY --chown=65532:65532 --from=build /out/garfex /garfex

USER 65532:65532
ENTRYPOINT ["/garfex"]
