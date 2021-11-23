################################
################################
## Dockerfile to run GH-Token ##
################################
################################

######################
# Pull in base image #
######################
FROM ubuntu:20.04 as final

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
    org.opencontainers.image.description="Convert GitHub App into usable PAT"

########################
# Specify workdir      #
########################
WORKDIR /app

########################
# Install dependencies #
########################
# hadolint ignore=DL3008
RUN apt-get update \
    && apt-get install --no-install-recommends -y \
    bash \
    ca-certificates \
    curl \
    git \
    jq \
    perl \
    vim \
    && rm -rf /var/lib/apt/lists/*

###########################
# Copy files to container #
###########################
COPY gh-token .
COPY .automation/run-gh-token.sh .

###################
# Install JWT-CLI #
###################
RUN curl -sL https://github.com/mike-engel/jwt-cli/releases/download/4.0.0/jwt-linux.tar.gz --output jwt-linux.tar.gz \
    && tar xvfz jwt-linux.tar.gz \
    && rm -f jwt-linux.tar.gz \
    && mv jwt /usr/bin

######################
# Set the entrypoint #
######################
ENTRYPOINT ["run-gh-token.sh"]
