# #####################################

#  base image sample to build an OCI image of your infra backend

# #####################################


# Stage 1: Build stage
FROM ubuntu:22.04 AS builder

# Update package lists and install necessary build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    wget \
    curl \
    git \
    unzip \
    jq \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Download and install Terraform
RUN wget https://releases.hashicorp.com/terraform/1.8.1/terraform_1.8.1_linux_amd64.zip && \
    unzip terraform_1.8.1_linux_amd64.zip -d /usr/local/bin/ && \
    rm terraform_1.8.1_linux_amd64.zip

## specific to your cloud implementation; replace with custom CLI
# Install AWS CLI
RUN curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip aws

# Stage 2: Runtime stage
FROM ubuntu:22.04

# Copy necessary binaries from the builder stage
COPY --from=builder /usr/local/bin/terraform /usr/local/bin/terraform
COPY --from=builder /usr/local/bin/aws /usr/local/bin/aws

# Clean up unnecessary packages from the runtime image
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*


WORKDIR /app

# Copy the rest of your application
COPY . .

CMD ["/bin/bash", "-c", "chmod +x $SCRIPT && exec $SCRIPT $ARGS"]
