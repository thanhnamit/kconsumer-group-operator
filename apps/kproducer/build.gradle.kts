import org.jetbrains.kotlin.gradle.tasks.KotlinCompile

plugins {
	id("org.springframework.boot") version "2.3.0.RELEASE"
	id("io.spring.dependency-management") version "1.0.9.RELEASE"
	id("com.google.cloud.tools.jib") version "2.3.0"
	kotlin("jvm") version "1.3.72"
	kotlin("plugin.spring") version "1.3.72"
}

group = "com.tna"
version = "0.1"
val author = "Nam Nguyen <thanhnam.it@gmail.com>"
val buildNumber by extra("0")
java.sourceCompatibility = JavaVersion.VERSION_11

repositories {
	mavenLocal()
	mavenCentral()
	jcenter()
}

dependencies {
	implementation("org.springframework.boot:spring-boot-starter-actuator")
	implementation("org.jetbrains.kotlin:kotlin-reflect")
	implementation("org.jetbrains.kotlin:kotlin-stdlib-jdk8")
	implementation("org.springframework.kafka:spring-kafka")
	implementation("io.micrometer:micrometer-registry-prometheus:latest.release")
	implementation("org.springframework.boot:spring-boot-starter-web")
  	implementation("com.fasterxml.jackson.module:jackson-module-kotlin")
	testImplementation("org.springframework.boot:spring-boot-starter-test") {
		exclude(group = "org.junit.vintage", module = "junit-vintage-engine")
	}
	testImplementation("org.springframework.kafka:spring-kafka-test")
}

tasks.withType<Test> {
	useJUnitPlatform()
}

tasks.withType<KotlinCompile> {
	kotlinOptions {
		freeCompilerArgs = listOf("-Xjsr305=strict")
		jvmTarget = "1.8"
	}
}

jib {
	to {
		image = "thenextapps/kproducer"
		tags = setOf("$version", "$version.${extra["buildNumber"]}")
	}

	container {
        labels = mapOf(
            "maintainer" to "$author",
            "org.opencontainers.image.title" to "kproducer",
            "org.opencontainers.image.description" to "An example Kafka producer",
            "org.opencontainers.image.version" to "$version",
            "org.opencontainers.image.authors" to "$author",
            "org.opencontainers.image.url" to "https://github.com/thanhnamit/kproducer",
            "org.opencontainers.image.vendor" to "https://thenextapps.com",
            "org.opencontainers.image.licenses" to "MIT"
        )
		ports = listOf("8085")
    }
}
