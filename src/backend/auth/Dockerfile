
FROM maven:3.9.11-eclipse-temurin-25 AS build
WORKDIR /src
COPY src/backend/auth/pom.xml ./
# Retry transient repository throttling and server errors during dependency prefetch.
# Maven 3.9.11's Resolver retries at most three times; persistent failures remain fatal.
RUN mvn -B \
    -Daether.connector.http.retryHandler.count=3 \
    -Daether.connector.http.retryHandler.serviceUnavailable=429,500,502,503,504 \
    dependency:go-offline
COPY protos /protos
COPY src/backend/auth/src ./src
RUN mvn -B package -DskipTests

FROM eclipse-temurin:25-jre
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*
RUN adduser --system --uid 65532 nonroot
USER nonroot
WORKDIR /
COPY --from=build /src/target/auth-0.1.0-SNAPSHOT.jar /auth.jar
EXPOSE 8080 9090
ENV SERVER_PORT=8080
ENTRYPOINT ["java", "-jar", "/auth.jar"]
