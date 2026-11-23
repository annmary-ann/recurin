# Production image for recurin: a static binary on distroless, running as a
# non-root user. The same image runs every role (api, worker, migrate).
#   docker build -t recurin .
#   docker run --rm -p 8080:8080 recurin api

FROM golang:1.27-bookworm AS build
WORKDIR /src

# Dependencies first, so code changes do not invalidate the module cache layer.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/recurin ./cmd/recurin

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/recurin /recurin
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/recurin"]
CMD ["api"]
