# Build the application using the Go 1.24 development container image
FROM registry.suse.com/bci/golang:1.24 as build

WORKDIR /smlm_sources

# pre-copy/cache go.mod for pre-downloading dependencies and only
# redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . ./

# Make sure to build the application with CGO disabled.
# This will force Go to use some Go implementations of code
# rather than those supplied by the host operating system.
# You need this for scratch images as those supporting libraries
# are not available.
RUN CGO_ENABLED=0 go build -o /smlm_tool

# Bundle the application into a scratch image
FROM scratch

COPY --from=build /smlm_tool /usr/local/bin/smlm_tool

CMD ["/usr/local/bin/smlm_tool"]