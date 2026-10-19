import com.google.protobuf.gradle.ProtobufExtension
import work.lclpnet.build.ext.BuildUtilsExtension

plugins {
    alias(libs.plugins.kotlin.jvm) apply false
    alias(libs.plugins.protobuf) apply false
    alias(libs.plugins.gradle.build.utils) apply false
}

subprojects {
    repositories {
        mavenCentral()
    }

    pluginManager.withPlugin(rootProject.libs.plugins.java.library.get().pluginId) {
        configure<BasePluginExtension> {
            archivesName = "ledger-${project.name}"
        }

        configure<JavaPluginExtension> {
            toolchain {
                languageVersion = JavaLanguageVersion.of(rootProject.libs.versions.java.get().toInt())
            }
        }

        pluginManager.withPlugin(rootProject.libs.plugins.protobuf.get().pluginId) {
            configure<SourceSetContainer> {
                named("main") {
                    extensions.getByName<SourceDirectorySet>("proto").srcDir(rootProject.file("../proto"))
                }
            }

            configure<ProtobufExtension> {
                protoc {
                    artifact = rootProject.libs.protoc.asProvider().get().toString()
                }
            }
        }

        pluginManager.withPlugin(rootProject.libs.plugins.maven.publish.get().pluginId) {
            pluginManager.withPlugin(rootProject.libs.plugins.gradle.build.utils.get().pluginId) {
                val buildUtils = the<BuildUtilsExtension>()
                val props = buildUtils.loadProperties("publish.properties")
                val displayName = project.name.replaceFirstChar { it.uppercase() }

                configure<PublishingExtension> {
                    publications {
                        create<MavenPublication>("mavenJava") {
                            artifactId = the<BasePluginExtension>().archivesName.get()

                            from(components["java"])

                            pom {
                                name = "Ledger $displayName"
                                description = "$displayName gRPC client library for the Ledger API."
                            }
                        }
                    }

                    buildUtils.setupPublishRepository(repositories, props)
                }
            }
        }
    }
}
