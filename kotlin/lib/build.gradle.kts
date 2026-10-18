import java.util.Properties

plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.java.library)
    alias(libs.plugins.protobuf)
    alias(libs.plugins.gradle.build.utils)
    alias(libs.plugins.maven.publish)
}

val props: Properties = buildUtils.loadProperties("publish.properties")

val javaVersion: Int = libs.versions.java.get().toInt()

base {
    archivesName = "ledger-kotlin"
}

repositories {
    mavenCentral()
}

sourceSets {
    main {
        proto {
            srcDir(rootProject.file("../proto"))
        }
    }
}

dependencies {
    implementation(libs.protovalidate)
    implementation(libs.google.common.protos)

    api(libs.protobuf.java.util)
    api(libs.protobuf.kotlin)
    api(libs.grpc.stub)
    api(libs.grpc.protobuf)
    api(libs.grpc.kotlin.stub)
    api(libs.kotlinx.coroutines.core)

    testImplementation(libs.kotlin.test)
    testImplementation(libs.junit.jupiter.engine)
    testRuntimeOnly(libs.junit.platform.launcher)
}

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(javaVersion)
    }
}

kotlin {
    jvmToolchain(javaVersion)
}

protobuf {
    protoc {
        artifact = libs.protoc.asProvider().get().toString()
    }

    plugins {
        create("grpc") {
            artifact = libs.protoc.gen.grpc.java.get().toString()
        }

        create("grpckt") {
            artifact = libs.protoc.gen.grpc.kotlin.get().toString() + ":jdk8@jar"
        }
    }

    generateProtoTasks {
        all().forEach {
            it.plugins {
                create("grpc")
                create("grpckt")
            }

            it.builtins {
                create("kotlin")
            }
        }
    }
}

tasks.named<Test>("test") {
    useJUnitPlatform()
}

publishing {
    publications {
        create<MavenPublication>("mavenJava") {
            artifactId = base.archivesName.get()

            from(components["java"])

            pom {
                name = "Ledger Kotlin"
                description = "Kotlin gRPC client library for the Ledger API."
            }
        }
    }

    buildUtils.setupPublishRepository(repositories, props)
}
