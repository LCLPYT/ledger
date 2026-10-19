plugins {
    alias(libs.plugins.java.library)
    alias(libs.plugins.protobuf)
    alias(libs.plugins.gradle.build.utils)
    alias(libs.plugins.maven.publish)
}

dependencies {
    implementation(libs.protovalidate)
    implementation(libs.google.common.protos)

    api(libs.protobuf.java.util)
    api(libs.grpc.stub)
    api(libs.grpc.protobuf)
}

tasks.withType<JavaCompile>().configureEach {
    options.release = libs.versions.jvm.target.java.get().toInt()
    options.compilerArgs.add("-Xlint:-options")
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
