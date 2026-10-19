import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.java.library)
    alias(libs.plugins.protobuf)
    alias(libs.plugins.gradle.build.utils)
    alias(libs.plugins.maven.publish)
}

val targetVersion: Int = libs.versions.jvm.target.core.get().toInt()

dependencies {
    implementation(libs.protovalidate)
    implementation(libs.google.common.protos)

    api(libs.protobuf.java.util)
    api(libs.grpc.stub)
    api(libs.grpc.protobuf)

    testImplementation(libs.kotlin.test)
    testImplementation(libs.junit.jupiter.engine)
    testRuntimeOnly(libs.junit.platform.launcher)
}

tasks.compileJava {
    options.release = targetVersion
    options.compilerArgs.add("-Xlint:-options")
}

kotlin {
    jvmToolchain(libs.versions.java.get().toInt())
    explicitApi()
}

// tests run on the toolchain JDK, only the published code targets the legacy JVM
tasks.compileKotlin {
    compilerOptions {
        val target = if (targetVersion == 8) "1.8" else targetVersion.toString()
        jvmTarget = JvmTarget.fromTarget(target)
        freeCompilerArgs.add("-Xjdk-release=$target")
    }
}

protobuf {
    plugins {
        create("grpc") {
            artifact = libs.protoc.gen.grpc.java.get().toString()
        }
    }

    generateProtoTasks {
        all().forEach {
            it.plugins {
                create("grpc")
            }
        }
    }
}

tasks.named<Test>("test") {
    useJUnitPlatform()
}
