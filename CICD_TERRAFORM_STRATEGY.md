# GitLab CI/CD & Terraform Strategy

**Document Type:** DevOps Implementation Strategy  
**Version:** 1.0.0  
**Date:** July 18, 2025  
**Platform:** keystone-gateway.dev

## 🎯 CI/CD Architecture Overview

### Pipeline Philosophy
- **Service-Aware**: Detect and deploy only changed services
- **Environment-Progressive**: Staging → Production with gates
- **Infrastructure-as-Code**: Terraform-managed infrastructure
- **Self-Hosting**: Use Keystone Gateway for platform routing

### Pipeline Architecture
```
GitLab Repository → CI/CD Pipeline → Infrastructure → Services
       ↓                ↓               ↓           ↓
   Code Changes    →  Build/Test   →  Terraform  →  Gateway Routing
   Documentation   →  Security     →  Provision  →  Load Balancing
   Configs         →  Deploy       →  Configure  →  Health Monitoring
```

## 🚀 GitLab CI/CD Implementation

### Main Pipeline Configuration
```yaml
# .gitlab-ci.yml
image: docker:24.0.5

variables:
  DOCKER_DRIVER: overlay2
  DOCKER_TLS_CERTDIR: "/certs"
  TERRAFORM_VERSION: "1.5.0"
  PLATFORM_REGISTRY: "$CI_REGISTRY/keystone-platform"

stages:
  - detect-changes
  - build-images
  - test-services
  - security-scan
  - deploy-staging
  - integration-test
  - manual-approval
  - deploy-production
  - post-deploy-validation

services:
  - docker:24.0.5-dind

before_script:
  - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY

# Change Detection Stage
detect-changes:
  stage: detect-changes
  image: alpine/git:latest
  script:
    - ci/scripts/detect-changes.sh
  artifacts:
    paths:
      - changed-services.txt
    expire_in: 1 hour

# Dynamic Service Build Jobs
.build-service:
  stage: build-images
  script:
    - SERVICE_NAME=$1
    - docker build -t $PLATFORM_REGISTRY/$SERVICE_NAME:$CI_COMMIT_SHA services/$SERVICE_NAME/
    - docker push $PLATFORM_REGISTRY/$SERVICE_NAME:$CI_COMMIT_SHA
    - docker tag $PLATFORM_REGISTRY/$SERVICE_NAME:$CI_COMMIT_SHA $PLATFORM_REGISTRY/$SERVICE_NAME:latest
    - docker push $PLATFORM_REGISTRY/$SERVICE_NAME:latest
  rules:
    - if: '$CI_PIPELINE_SOURCE == "push"'
      changes:
        - services/$SERVICE_NAME/**/*

# Service-Specific Build Jobs
build-gateway:
  extends: .build-service
  variables:
    SERVICE_NAME: gateway
  rules:
    - changes:
        - services/gateway/**/*

build-blog:
  extends: .build-service
  variables:
    SERVICE_NAME: blog
  rules:
    - changes:
        - services/blog/**/*

build-playground:
  extends: .build-service
  variables:
    SERVICE_NAME: playground
  rules:
    - changes:
        - services/playground/**/*

build-docs:
  extends: .build-service
  variables:
    SERVICE_NAME: docs
  rules:
    - changes:
        - services/docs/**/*

# Testing Stage
.test-service:
  stage: test-services
  script:
    - cd services/$SERVICE_NAME
    - ci/scripts/test-service.sh $SERVICE_NAME

test-gateway:
  extends: .test-service
  variables:
    SERVICE_NAME: gateway
  script:
    - cd services/gateway
    - go test -v ./...
    - go test -race ./...
    - go test -cover ./...

test-playground:
  extends: .test-service
  variables:
    SERVICE_NAME: playground
  script:
    - cd services/playground
    - npm ci
    - npm run test
    - npm run lint

# Security Scanning
security-scan:
  stage: security-scan
  image: docker:24.0.5
  script:
    - ci/scripts/security-scan.sh
  artifacts:
    reports:
      sast: gl-sast-report.json
    expire_in: 1 week

# Infrastructure Deployment
deploy-infrastructure-staging:
  stage: deploy-staging
  image: hashicorp/terraform:$TERRAFORM_VERSION
  script:
    - cd infrastructure/terraform/environments/staging
    - terraform init
    - terraform plan -out=tfplan
    - terraform apply tfplan
  artifacts:
    paths:
      - infrastructure/terraform/environments/staging/tfplan
    expire_in: 1 week
  environment:
    name: staging-infrastructure
    url: https://staging.keystone-gateway.dev

# Service Deployment
deploy-services-staging:
  stage: deploy-staging
  image: docker:24.0.5
  script:
    - ci/scripts/deploy-services.sh staging
  environment:
    name: staging
    url: https://staging.keystone-gateway.dev
  dependencies:
    - deploy-infrastructure-staging

# Integration Testing
integration-test-staging:
  stage: integration-test
  image: alpine:latest
  script:
    - apk add --no-cache curl jq
    - ci/scripts/integration-test.sh staging
  artifacts:
    reports:
      junit: integration-test-results.xml

# Manual Approval Gate
manual-approval:
  stage: manual-approval
  script:
    - echo "Manual approval required for production deployment"
  when: manual
  only:
    - main

# Production Deployment
deploy-infrastructure-production:
  stage: deploy-production
  image: hashicorp/terraform:$TERRAFORM_VERSION
  script:
    - cd infrastructure/terraform/environments/production
    - terraform init
    - terraform plan -out=tfplan
    - terraform apply tfplan
  environment:
    name: production-infrastructure
    url: https://keystone-gateway.dev
  when: manual
  only:
    - main

deploy-services-production:
  stage: deploy-production
  image: docker:24.0.5
  script:
    - ci/scripts/deploy-services.sh production
  environment:
    name: production
    url: https://keystone-gateway.dev
  dependencies:
    - deploy-infrastructure-production
  when: manual
  only:
    - main

# Post-Deployment Validation
post-deploy-validation:
  stage: post-deploy-validation
  image: alpine:latest
  script:
    - apk add --no-cache curl
    - ci/scripts/post-deploy-validation.sh production
  dependencies:
    - deploy-services-production
```

### Service Detection Script
```bash
#!/bin/bash
# ci/scripts/detect-changes.sh

# Detect changed services based on git diff
CHANGED_SERVICES=""

# Get list of changed files
CHANGED_FILES=$(git diff --name-only $CI_COMMIT_BEFORE_SHA $CI_COMMIT_SHA)

# Check each service directory
for service in gateway website blog playground docs monitoring; do
    if echo "$CHANGED_FILES" | grep -q "services/$service/"; then
        CHANGED_SERVICES="$CHANGED_SERVICES $service"
        echo "Detected changes in service: $service"
    fi
done

# Check infrastructure changes
if echo "$CHANGED_FILES" | grep -q "infrastructure/"; then
    echo "infrastructure" >> changed-components.txt
    echo "Detected infrastructure changes"
fi

# Output changed services
echo "$CHANGED_SERVICES" > changed-services.txt
echo "Changed services: $CHANGED_SERVICES"
```

## 🏗️ Terraform Infrastructure Strategy

### Infrastructure Architecture
```
AWS Cloud Infrastructure
├── VPC (Virtual Private Cloud)
│   ├── Public Subnets (ALB, NAT Gateway)
│   └── Private Subnets (ECS Tasks, RDS)
├── ECS Cluster (Container Orchestration)
│   ├── Service Tasks (Auto Scaling)
│   └── Task Definitions (Service Configs)
├── Application Load Balancer
│   ├── Target Groups (Service Routing)
│   └── Listeners (HTTPS Termination)
├── Route 53 (DNS Management)
│   ├── Hosted Zone (keystone-gateway.dev)
│   └── Records (Service Subdomains)
└── Monitoring (CloudWatch + Grafana)
    ├── Logs (Centralized Logging)
    └── Metrics (Performance Monitoring)
```

### Root Infrastructure Configuration
```hcl
# infrastructure/terraform/main.tf
terraform {
  required_version = ">= 1.5.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  
  backend "s3" {
    bucket = "keystone-platform-terraform-state"
    key    = "platform/terraform.tfstate"
    region = "eu-central-1"
  }
}

provider "aws" {
  region = var.aws_region
  
  default_tags {
    tags = {
      Project     = "keystone-platform"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# VPC Module
module "vpc" {
  source = "./modules/vpc"
  
  environment = var.environment
  cidr_block  = var.vpc_cidr
  
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
  availability_zones   = var.availability_zones
}

# ECS Cluster Module
module "ecs" {
  source = "./modules/ecs"
  
  environment    = var.environment
  vpc_id         = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  public_subnet_ids  = module.vpc.public_subnet_ids
  
  services = var.services
}

# Application Load Balancer Module
module "alb" {
  source = "./modules/alb"
  
  environment = var.environment
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.public_subnet_ids
  
  domain_name = var.domain_name
  services    = var.services
}

# Route 53 DNS Module
module "route53" {
  source = "./modules/route53"
  
  domain_name = var.domain_name
  alb_dns_name = module.alb.dns_name
  alb_zone_id  = module.alb.zone_id
  
  services = var.services
}

# Monitoring Module
module "monitoring" {
  source = "./modules/monitoring"
  
  environment = var.environment
  vpc_id      = module.vpc.vpc_id
  subnet_ids  = module.vpc.private_subnet_ids
  
  ecs_cluster_name = module.ecs.cluster_name
  services         = var.services
}
```

### ECS Service Module
```hcl
# infrastructure/terraform/modules/ecs/main.tf
resource "aws_ecs_cluster" "platform" {
  name = "keystone-platform-${var.environment}"
  
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
  
  tags = {
    Name = "keystone-platform-${var.environment}"
  }
}

# Gateway Service Task Definition
resource "aws_ecs_task_definition" "gateway" {
  family                   = "keystone-gateway-${var.environment}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "512"
  memory                   = "1024"
  execution_role_arn       = aws_iam_role.ecs_execution_role.arn
  task_role_arn           = aws_iam_role.ecs_task_role.arn
  
  container_definitions = jsonencode([
    {
      name  = "gateway"
      image = "${var.registry_url}/gateway:${var.image_tag}"
      
      portMappings = [
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]
      
      environment = [
        {
          name  = "ENV"
          value = var.environment
        }
      ]
      
      healthCheck = {
        command = ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1"]
        interval = 30
        timeout = 10
        retries = 3
        startPeriod = 60
      }
      
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.platform.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "gateway"
        }
      }
    }
  ])
}

# Gateway ECS Service
resource "aws_ecs_service" "gateway" {
  name            = "gateway-${var.environment}"
  cluster         = aws_ecs_cluster.platform.id
  task_definition = aws_ecs_task_definition.gateway.arn
  desired_count   = var.environment == "production" ? 2 : 1
  launch_type     = "FARGATE"
  
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = false
  }
  
  load_balancer {
    target_group_arn = var.gateway_target_group_arn
    container_name   = "gateway"
    container_port   = 8080
  }
  
  depends_on = [var.alb_listener]
  
  deployment_configuration {
    maximum_percent         = 200
    minimum_healthy_percent = 100
  }
  
  tags = {
    Name = "gateway-${var.environment}"
  }
}

# Auto Scaling
resource "aws_appautoscaling_target" "gateway" {
  max_capacity       = var.environment == "production" ? 10 : 3
  min_capacity       = var.environment == "production" ? 2 : 1
  resource_id        = "service/${aws_ecs_cluster.platform.name}/${aws_ecs_service.gateway.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "gateway_cpu" {
  name               = "gateway-cpu-scaling-${var.environment}"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.gateway.resource_id
  scalable_dimension = aws_appautoscaling_target.gateway.scalable_dimension
  service_namespace  = aws_appautoscaling_target.gateway.service_namespace
  
  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value = 70.0
  }
}
```

### Environment-Specific Configurations

#### Staging Environment
```hcl
# infrastructure/terraform/environments/staging/terraform.tfvars
environment = "staging"
aws_region  = "eu-central-1"

# Network Configuration
vpc_cidr             = "10.1.0.0/16"
public_subnet_cidrs  = ["10.1.1.0/24", "10.1.2.0/24"]
private_subnet_cidrs = ["10.1.10.0/24", "10.1.20.0/24"]
availability_zones   = ["eu-central-1a", "eu-central-1b"]

# Domain Configuration
domain_name = "staging.keystone-gateway.dev"

# Service Configuration
services = {
  gateway = {
    image     = "keystone-platform/gateway"
    cpu       = 512
    memory    = 1024
    count     = 1
    port      = 8080
    health    = "/health"
  }
  blog = {
    image     = "keystone-platform/blog"
    cpu       = 256
    memory    = 512
    count     = 1
    port      = 3000
    health    = "/health"
  }
  playground = {
    image     = "keystone-platform/playground"
    cpu       = 512
    memory    = 1024
    count     = 1
    port      = 3000
    health    = "/api/health"
  }
}
```

#### Production Environment
```hcl
# infrastructure/terraform/environments/production/terraform.tfvars
environment = "production"
aws_region  = "eu-central-1"

# Network Configuration
vpc_cidr             = "10.0.0.0/16"
public_subnet_cidrs  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
private_subnet_cidrs = ["10.0.10.0/24", "10.0.20.0/24", "10.0.30.0/24"]
availability_zones   = ["eu-central-1a", "eu-central-1b", "eu-central-1c"]

# Domain Configuration
domain_name = "keystone-gateway.dev"

# Service Configuration
services = {
  gateway = {
    image     = "keystone-platform/gateway"
    cpu       = 1024
    memory    = 2048
    count     = 2
    port      = 8080
    health    = "/health"
  }
  blog = {
    image     = "keystone-platform/blog"
    cpu       = 512
    memory    = 1024
    count     = 2
    port      = 3000
    health    = "/health"
  }
  playground = {
    image     = "keystone-platform/playground"
    cpu       = 1024
    memory    = 2048
    count     = 2
    port      = 3000
    health    = "/api/health"
  }
}
```

## 🔧 Deployment Scripts

### Service Deployment Script
```bash
#!/bin/bash
# ci/scripts/deploy-services.sh

ENVIRONMENT=$1
CHANGED_SERVICES_FILE="changed-services.txt"

if [[ "$ENVIRONMENT" != "staging" && "$ENVIRONMENT" != "production" ]]; then
    echo "Error: Environment must be 'staging' or 'production'"
    exit 1
fi

echo "Deploying services to $ENVIRONMENT environment..."

# Read changed services
if [[ -f "$CHANGED_SERVICES_FILE" ]]; then
    CHANGED_SERVICES=$(cat $CHANGED_SERVICES_FILE)
    echo "Changed services: $CHANGED_SERVICES"
else
    echo "No changed services detected, deploying all services"
    CHANGED_SERVICES="gateway website blog playground docs monitoring"
fi

# Deploy each changed service
for service in $CHANGED_SERVICES; do
    echo "Deploying service: $service"
    
    # Update ECS service with new task definition
    aws ecs update-service \
        --cluster "keystone-platform-$ENVIRONMENT" \
        --service "$service-$ENVIRONMENT" \
        --task-definition "$service-$ENVIRONMENT:$CI_PIPELINE_ID" \
        --region eu-central-1
    
    # Wait for deployment to complete
    aws ecs wait services-stable \
        --cluster "keystone-platform-$ENVIRONMENT" \
        --services "$service-$ENVIRONMENT" \
        --region eu-central-1
    
    echo "Service $service deployed successfully"
done

echo "All services deployed to $ENVIRONMENT"
```

### Integration Testing Script
```bash
#!/bin/bash
# ci/scripts/integration-test.sh

ENVIRONMENT=$1
BASE_URL="https://$ENVIRONMENT.keystone-gateway.dev"

if [[ "$ENVIRONMENT" == "production" ]]; then
    BASE_URL="https://keystone-gateway.dev"
fi

echo "Running integration tests against $BASE_URL"

# Test gateway health
echo "Testing gateway health..."
curl -f "$BASE_URL/health" || exit 1

# Test service routing
echo "Testing blog service routing..."
curl -f "$BASE_URL/blog/health" || exit 1

echo "Testing playground service routing..."
curl -f "$BASE_URL/playground/api/health" || exit 1

echo "Testing docs service routing..."
curl -f "$BASE_URL/docs/health" || exit 1

# Test load balancing (if multiple instances)
if [[ "$ENVIRONMENT" == "production" ]]; then
    echo "Testing load balancing..."
    for i in {1..10}; do
        response=$(curl -s "$BASE_URL/api/instance-id")
        echo "Request $i: Instance $response"
    done
fi

echo "All integration tests passed!"
```

## 📊 Monitoring & Observability

### Terraform Monitoring Configuration
```hcl
# infrastructure/terraform/modules/monitoring/main.tf
resource "aws_cloudwatch_log_group" "platform" {
  name              = "/ecs/keystone-platform-${var.environment}"
  retention_in_days = var.environment == "production" ? 30 : 7
  
  tags = {
    Name = "keystone-platform-${var.environment}"
  }
}

# CloudWatch Dashboard
resource "aws_cloudwatch_dashboard" "platform" {
  dashboard_name = "keystone-platform-${var.environment}"
  
  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        
        properties = {
          metrics = [
            ["AWS/ECS", "CPUUtilization", "ServiceName", "gateway-${var.environment}"],
            [".", "MemoryUtilization", ".", "."],
          ]
          period = 300
          stat   = "Average"
          region = var.aws_region
          title  = "Gateway Service Metrics"
        }
      }
    ]
  })
}

# CloudWatch Alarms
resource "aws_cloudwatch_metric_alarm" "gateway_cpu_high" {
  alarm_name          = "gateway-cpu-high-${var.environment}"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = "300"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors gateway CPU utilization"
  
  dimensions = {
    ServiceName = "gateway-${var.environment}"
  }
  
  alarm_actions = [aws_sns_topic.alerts.arn]
}
```

## 🎯 Implementation Timeline

### Week 1: CI/CD Foundation
- [ ] Setup GitLab CI pipeline structure
- [ ] Implement service detection logic
- [ ] Create basic deployment scripts
- [ ] Configure container registry

### Week 2: Terraform Infrastructure
- [ ] Design VPC and networking
- [ ] Implement ECS cluster configuration
- [ ] Setup load balancer and DNS
- [ ] Configure monitoring infrastructure

### Week 3: Service Integration
- [ ] Implement service-specific pipelines
- [ ] Configure auto-scaling policies
- [ ] Setup health monitoring
- [ ] Implement blue/green deployment

### Week 4: Production Readiness
- [ ] Security scanning integration
- [ ] Performance testing automation
- [ ] Disaster recovery procedures
- [ ] Documentation and runbooks

This CI/CD and Terraform strategy provides a robust, scalable foundation for the keystone-gateway.dev platform while demonstrating enterprise-grade DevOps practices.
