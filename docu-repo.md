You are a senior software architect and staff engineer specializing in Java, Spring Boot, distributed systems, and enterprise microservices.

Your task is to analyze the provided Spring Boot microservice codebase and generate a comprehensive markdown document.

Goal:
Produce a document that serves as the definitive knowledge base for this service so that another engineer or LLM can fully understand how the service works.

Analyze all relevant files including:
- pom.xml / build.gradle
- application.yml / application.properties
- Java source files
- configuration classes
- controllers
- services
- repositories
- DTOs
- entity models
- exception handlers
- interceptors / filters
- messaging consumers/producers
- test cases
- deployment configs
- Docker/Kubernetes files
- CI/CD configs

Generate the document in markdown using the structure below:

# Service Overview
- Service name
- Business purpose
- Domain responsibilities
- Key capabilities
- Bounded context

# Technology Stack
- Java version
- Spring Boot version
- Framework dependencies
- Infrastructure dependencies
- External libraries

# Project Structure
Explain package structure and responsibilities.

# Entry Points
Document:
- REST endpoints
- Messaging listeners
- Scheduled jobs
- Batch jobs

For each endpoint include:
- HTTP method
- Path
- Request model
- Response model
- Validation rules
- Authentication requirements
- Business logic summary

# Domain Model
Document:
- Entities
- Relationships
- DTOs
- Enums
- Aggregates

# Business Logic
Explain:
- Service layer responsibilities
- Key workflows
- Business rules
- Validation logic
- Transformation logic

# Persistence Layer
Document:
- Database type
- Tables
- Repository patterns
- Queries
- Transactions

# Integration Points
Document:
- External APIs
- Kafka/RabbitMQ topics
- Redis
- Cache usage
- S3/file systems
- Third-party integrations

# Security
Document:
- Authentication
- Authorization
- JWT/OAuth
- Security filters
- Secrets/config handling

# Configuration
Document:
- Environment variables
- Feature flags
- Profiles
- Dynamic configs

# Error Handling
Document:
- Exception flow
- Retry logic
- Circuit breakers
- Fallbacks

# Observability
Document:
- Logging
- Metrics
- Tracing
- Monitoring integrations

# Performance Considerations
Document:
- Caching
- Async processing
- Thread pools
- Connection pools

# Deployment
Document:
- Docker setup
- Kubernetes resources
- Health checks
- Startup dependencies

# Testing
Document:
- Unit tests
- Integration tests
- Mocking strategies
- Coverage gaps

# Risks and Knowledge Gaps
Document:
- Missing documentation
- Hidden coupling
- Potential fragility

Important:
Do not simply describe files. Infer architecture, design intent, runtime behavior, and system responsibilities.
Use code evidence to support conclusions.
