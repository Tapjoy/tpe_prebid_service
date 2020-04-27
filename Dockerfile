ARG GO_IMAGE
###################
### Base Image  ###
###################
FROM ${GO_IMAGE} as baseimage

# Install OS-level language locales
ENV DEBIAN_FRONTEND=noninteractive LANG=en_US.UTF-8 LANGUAGE=en_US:en LC_ALL=en_US.UTF-8
RUN apt-get update -q &&\
    apt-get install -yq --no-install-recommends locales locales-all &&\
    locale-gen $LANG && update-locale LANG=$LANG &&\
    rm -rf /var/lib/apt/lists/* /tmp/*

# We install OS-level dependencies we need to work with the project
RUN apt-get update -q &&\
    apt-get install -y --no-install-recommends vim &&\
    rm -rf /var/lib/apt/lists/* /tmp/*

WORKDIR /go/src/github.com/tapjoy/tpe_prebid_service

###################
# Build-time prep #
###################

FROM baseimage as artifact-prep

# Copy local-to-builder files and folders into current directory (WORKDIR) of the container
ADD . .

# Remove untracked files and folders
# Run artifact preparation steps (e.g. geoip, bundle install, etc)
# Clean up
RUN git clean -fxd &&\
    make artifact-prep &&\
    rm -rf .git /tmp/*

###################
# Artifact target #
###################

FROM baseimage as artifact
COPY --from=artifact-prep /go/src/github.com/tapjoy/tpe_prebid_service /project
WORKDIR /project

# EXPOSE 8000
# EXPOSE 8080
