FROM docker.io/library/golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/index.html docs/embed.go docs/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /studio-map ./cmd/studio-map

FROM scratch
COPY --from=build /studio-map /studio-map
USER 65532:65532
ENV STUDIO_MAP_CONFIG=/etc/studio-map/studio_config.json
EXPOSE 8080
ENTRYPOINT ["/studio-map", "serve", "--addr", ":8080"]
