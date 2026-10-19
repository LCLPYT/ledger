import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.java.library)
    alias(libs.plugins.protobuf)
    alias(libs.plugins.gradle.build.utils)
    alias(libs.plugins.maven.publish)
}

val targetVersion: Int = libs.versions.jvm.target.kotlin.get().toInt()

dependencies {
    api(project(":java"))
    api(libs.protobuf.kotlin)
    api(libs.grpc.kotlin.stub)
    api(libs.kotlinx.coroutines.core)

    testImplementation(libs.kotlin.test)
    testImplementation(libs.junit.jupiter.engine)
    testRuntimeOnly(libs.junit.platform.launcher)
}

tasks.withType<JavaCompile>().configureEach {
    options.release = targetVersion
}

kotlin {
    jvmToolchain(libs.versions.java.get().toInt())

    compilerOptions {
        jvmTarget = JvmTarget.fromTarget(targetVersion.toString())  // bytecode version target
        freeCompilerArgs.add("-Xjdk-release=$targetVersion")        // limit visible apis of newer compile JDKs
    }
}

protobuf {
    plugins {
        create("grpckt") {
            artifact = libs.protoc.gen.grpc.kotlin.get().toString() + ":jdk8@jar"
        }
    }

    generateProtoTasks {
        all().forEach {
            it.plugins {
                create("grpckt")
            }

            it.builtins {
                remove(getByName("java"))
                create("kotlin")
            }
        }
    }
}

// proto files are already contained within the java artifact, which is a dependency of the kotlin artifact
tasks.processResources {
    exclude("**/*.proto")
}

tasks.named<Test>("test") {
    useJUnitPlatform()
}
