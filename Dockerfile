FROM golang:1.27.2 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /libratrack ./cmd/api

FROM scratch
COPY --from=build /libratrack /libratrack
ENV LISTEN_HOST=0.0.0.0
EXPOSE 8085
USER 65532:65532
ENTRYPOINT ["/libratrack"]
