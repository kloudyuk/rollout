FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN GOOS=linux go build -o rollout

FROM gcr.io/distroless/static-debian12
COPY --from=build /app/rollout /
CMD ["/rollout"]
