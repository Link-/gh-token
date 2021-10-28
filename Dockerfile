################################
################################
## Dockerfile to run GH-Token ##
################################
################################

######################
# Pull in base image #
######################
FROM alpine:3.14.2 as final

###########################
# Get the build arguments #
###########################
ARG BUILD_DATE
ARG BUILD_REVISION
ARG BUILD_VERSION

#################################################
# Set ENV values used for debugging the version #
#################################################
ENV BUILD_DATE=$BUILD_DATE
ENV BUILD_REVISION=$BUILD_REVISION
ENV BUILD_VERSION=$BUILD_VERSION

#########################################
# Label the instance and set maintainer #
#########################################
LABEL com.github.actions.name="GH-Token" \
    com.github.actions.description="Convert GitHub App Auth into PAT" \
    com.github.actions.icon="code" \
    com.github.actions.color="blue" \
    maintainer="GitHub DevOps <github_devops@github.com>" \
    org.opencontainers.image.created=$BUILD_DATE \
    org.opencontainers.image.revision=$BUILD_REVISION \
    org.opencontainers.image.version=$BUILD_VERSION \
    org.opencontainers.image.authors="GitHub DevOps <github_devops@github.com>" \
    org.opencontainers.image.url="https://github.com/link-/gh-token" \
    org.opencontainers.image.source="https://github.com/link-/gh-token" \
    org.opencontainers.image.documentation="https://github.com/link-/gh-tokenr" \
    org.opencontainers.image.vendor="GitHub" \
    org.opencontainers.image.description="Convert GitHub App Auth into PAT"

########################
# Install dependencies #
########################
RUN apk add --no-cache \
    bash \
    git \
    jq \
    perl

###########################
# Copy files to container #
###########################
COPY gh-token /

######################
# Set the entrypoint #
######################
ENTRYPOINT ["/bin/bash"]
