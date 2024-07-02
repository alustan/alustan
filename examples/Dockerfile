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
    && rm -rf /var/lib/apt/lists/*

# Define Terraform version and download URL
ENV TERRAFORM_VERSION=1.8.1
ENV TERRAFORM_URL=https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip

# Download and install Terraform
RUN curl -fsSL ${TERRAFORM_URL} -o terraform.zip && \
    unzip terraform.zip -d /usr/local/bin/ && \
    rm terraform.zip

# Install kubectl
RUN curl -fsSL "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o kubectl && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm kubectl

## specific to your cloud implementaion; replace with custom cli
# Install AWS CLI
RUN curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip aws

# Stage 2: Runtime stage
FROM ubuntu:22.04

# Copy necessary binaries from the builder stage
COPY --from=builder /usr/local/bin/terraform /usr/local/bin/terraform
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/aws /usr/local/bin/aws

# Clean up unnecessary packages from the runtime image
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Set up a non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser
USER appuser

WORKDIR /app

# Copy the rest of your application
COPY . .

CMD ["/bin/bash", "-c", "chmod +x $SCRIPT && exec $SCRIPT"]



