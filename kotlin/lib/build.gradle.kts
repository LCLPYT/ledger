plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.java.library)
}

val javaVersion = libs.versions.java.get()

repositories {
    mavenCentral()
}

dependencies {
    testImplementation(libs.kotlin.test)
    testImplementation(libs.junit.jupiter.engine)
    testRuntimeOnly(libs.junit.platform.launcher)
}

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(javaVersion)
    }
}

tasks.named<Test>("test") {
    useJUnitPlatform()
}
