plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.river.tv"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.river.tv"
        // 23 / Android 6.0 covers virtually every Android TV + Fire TV
        // device shipped in the last 8 years and keeps WebView features
        // we rely on (modern JS, EME, fetch, fullscreen).
        minSdk = 23
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
        debug {
            isMinifyEnabled = false
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        viewBinding = true
    }

    // The river-tv Vite build lands in build/generated/river-tv-webapp/,
    // which we register here so mergeAssets picks it up alongside
    // src/main/assets. Nothing is committed under src/main/assets —
    // the entire web bundle is a build artifact.
    sourceSets["main"].assets.srcDir(
        layout.buildDirectory.dir("generated/river-tv-webapp"),
    )
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    // WebViewAssetLoader lets us serve the bundled Vite build under a
    // synthetic https://appassets.androidplatform.net/ origin, so
    // fetch/EME/storage all get a real HTTPS origin instead of the
    // file:// origin they'd get from a raw asset load.
    implementation("androidx.webkit:webkit:1.11.0")
    // Leanback brings the TV launcher intent + standard styles.
    implementation("androidx.leanback:leanback:1.0.0")
}

// -----------------------------------------------------------------
// river-tv web bundle → APK assets
//
// preBuild depends on riverTvSyncAssets, which depends on riverTvBuild,
// which depends on riverTvNpmInstall. Each task declares its inputs
// and outputs so Gradle can skip re-running when nothing changed.
//
// Requires `npm` on PATH at build time. This is a dev-machine
// prerequisite — the CI/release story lives in the README.
// -----------------------------------------------------------------

val riverTvDir = rootProject.file("../river-tv")
val webappGeneratedDir = layout.buildDirectory.dir("generated/river-tv-webapp/webapp")

val riverTvNpmInstall = tasks.register<Exec>("riverTvNpmInstall") {
    group = "river-tv"
    description = "Install river-tv npm dependencies."
    workingDir = riverTvDir
    commandLine("npm", "ci", "--prefer-offline", "--no-audit", "--no-fund")
    inputs.file(riverTvDir.resolve("package.json"))
    inputs.file(riverTvDir.resolve("package-lock.json"))
    // npm rewrites this file whenever it touches node_modules, so it's
    // a reliable up-to-date marker without depending on the entire
    // node_modules tree (which npm mutates on every invocation).
    outputs.file(riverTvDir.resolve("node_modules/.package-lock.json"))
}

val riverTvBuild = tasks.register<Exec>("riverTvBuild") {
    group = "river-tv"
    description = "Build the river-tv Vite bundle (dist/)."
    dependsOn(riverTvNpmInstall)
    workingDir = riverTvDir
    commandLine("npm", "run", "build")
    inputs.file(riverTvDir.resolve("package.json"))
    inputs.file(riverTvDir.resolve("vite.config.ts"))
    inputs.file(riverTvDir.resolve("index.html"))
    inputs.file(riverTvDir.resolve("tsconfig.json"))
    inputs.file(riverTvDir.resolve("tsconfig.app.json"))
    inputs.dir(riverTvDir.resolve("src"))
    outputs.dir(riverTvDir.resolve("dist"))
}

val riverTvSyncAssets = tasks.register<Sync>("riverTvSyncAssets") {
    group = "river-tv"
    description = "Copy river-tv dist/ into the APK's generated assets."
    dependsOn(riverTvBuild)
    from(riverTvDir.resolve("dist"))
    into(webappGeneratedDir)
}

tasks.named("preBuild") {
    dependsOn(riverTvSyncAssets)
}
