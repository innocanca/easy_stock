FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/server /server
ENV PORT=4000
EXPOSE 4000
ENTRYPOINT ["/server"]
