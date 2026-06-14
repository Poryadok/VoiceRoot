# syntax=docker/dockerfile:1

FROM maven:3.9.11-eclipse-temurin-25 AS build
WORKDIR /src
COPY pom.xml ./
RUN mvn -B dependency:go-offline
COPY src ./src
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
