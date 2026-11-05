import com.android.build.gradle.LibraryExtension
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    kotlin("multiplatform")
    kotlin("plugin.serialization")
    id("com.google.devtools.ksp")
    id("androidx.room")
}

val jvmOnly = providers.gradleProperty("pocJvmOnly").orNull == "true"
if (!jvmOnly) {
    apply(plugin = "com.android.library")
    extensions.configure<LibraryExtension> {
        namespace = "org.openalc.relay.poc.core"
        compileSdk = 36
        defaultConfig { minSdk = 26 }
        compileOptions {
            sourceCompatibility = JavaVersion.VERSION_17
            targetCompatibility = JavaVersion.VERSION_17
        }
    }
}

kotlin {
    compilerOptions { freeCompilerArgs.add("-Xexpect-actual-classes") }
    jvm {
        compilerOptions { jvmTarget.set(JvmTarget.JVM_17) }
        testRuns["test"].executionTask.configure {
            jvmArgs("--add-modules", "jdk.httpserver")
        }
    }
    if (!jvmOnly) {
        androidTarget {
            compilerOptions { jvmTarget.set(JvmTarget.JVM_17) }
        }
    }
    sourceSets {
        commonMain.dependencies {
            implementation("androidx.room:room-runtime:2.8.5")
            implementation("androidx.sqlite:sqlite-bundled:2.6.2")
            implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core:1.10.2")
            implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.9.0")
        }
        jvmMain { kotlin.srcDir("src/jvmSharedMain/kotlin") }
        if (!jvmOnly) {
            getByName("androidMain").kotlin.srcDir("src/jvmSharedMain/kotlin")
        }
        jvmTest.dependencies {
            implementation(kotlin("test-junit"))
        }
    }
}

dependencies {
    add("kspJvm", "androidx.room:room-compiler:2.8.5")
    if (!jvmOnly) add("kspAndroid", "androidx.room:room-compiler:2.8.5")
}

room { schemaDirectory("$projectDir/schemas") }

tasks.register<JavaExec>("pocHarness") {
    val compilation = kotlin.targets.getByName("jvm").compilations.getByName("main")
    dependsOn(compilation.compileTaskProvider)
    classpath(compilation.output.allOutputs, compilation.runtimeDependencyFiles)
    mainClass.set("org.openalc.relay.poc.HarnessKt")
    args(
        providers.gradleProperty("pocMode").orElse("sync").get(),
        providers.gradleProperty("pocDatabase").orElse("build/poc-harness.db").get(),
        providers.gradleProperty("pocEndpoint").orElse("http://127.0.0.1:8787/poc/v1/events").get(),
        providers.gradleProperty("pocCount").orElse("100").get(),
    )
}
