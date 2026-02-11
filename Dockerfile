FROM golang:1.22.3-alpine3.18 as build

RUN apk add --no-cache gcc musl-dev alpine-sdk

WORKDIR /app

COPY . .

RUN go mod tidy

RUN CGO_ENABLED=1 make build

FROM alpine:3.18

RUN apk add --no-cache curl tar

ENV TRIVY_VERSION=0.53.0

# RUN curl -L https://github.com/aquasecurity/trivy/releases/latest/download/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz -o trivy.tar.gz && \
#     tar -xzf trivy.tar.gz && \
#     mv trivy /usr/local/bin/trivy && \
#     rm trivy.tar.gz

RUN curl -L https://github.com/aquasecurity/trivy/releases/download/v0.53.0/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz -o trivy.tar.gz && \
    tar -xzf trivy.tar.gz && \
    mv trivy /usr/local/bin/trivy && \
    rm trivy.tar.gz

COPY --from=build /app/defender /defender

EXPOSE 3000

CMD ["/defender", "-gui"]
