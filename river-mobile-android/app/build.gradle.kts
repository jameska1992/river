plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.river.mobile"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.river.mobile"
        // 23 / Android 6.0 keeps the WebView features we rely on (modern JS,
        // Media Session, fetch, fullscreen) while covering nearly every phone
        // still in use.
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

    // The river-mobile Vite build lands in build/generated/river-mobile-webapp/,
    // registered here so mergeAssets picks it up. Nothing is committed under
    // src/main/assets — the whole web bundle is a build artifact.
    sourceSets["main"].assets.srcDir(
        layout.buildDirectory.dir("generated/river-mobile-webapp"),
    )
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
    // WebViewAssetLoader serves the bundled Vite build under a synthetic
    // appassets.androidplatform.net origin, so fetch / Media Session / storage
    // get a real origin instead of file://.
    implementation("androidx.webkit:webkit:1.11.0")
    // No leanback here — this is the phone/tablet build (river-tv-android is
    // the TV counterpart).
}

// -----------------------------------------------------------------
// river-mobile web bundle → APK assets
//
// preBuild depends on riverMobileSyncAssets → riverMobileBuild →
// riverMobileNpmInstall. Each task declares inputs/outputs so Gradle skips
// re-running when nothing changed. Requires `npm` on PATH at build time
// (a dev-machine prerequisite — see the README for the CI/release story).
// -----------------------------------------------------------------

val riverMobileDir = rootProject.file("../river-mobile")
val webappGeneratedDir = layout.buildDirectory.dir("generated/river-mobile-webapp/webapp")

val riverMobileNpmInstall = tasks.register<Exec>("riverMobileNpmInstall") {
    group = "river-mobile"
    description = "Install river-mobile npm dependencies."
    workingDir = riverMobileDir
    commandLine("npm", "ci", "--prefer-offline", "--no-audit", "--no-fund")
    inputs.file(riverMobileDir.resolve("package.json"))
    inputs.file(riverMobileDir.resolve("package-lock.json"))
    // npm rewrites this on any node_modules change — a reliable up-to-date
    // marker without depending on the whole tree (which npm mutates each run).
    outputs.file(riverMobileDir.resolve("node_modules/.package-lock.json"))
}

val riverMobileBuild = tasks.register<Exec>("riverMobileBuild") {
    group = "river-mobile"
    description = "Build the river-mobile Vite bundle (dist/)."
    dependsOn(riverMobileNpmInstall)
    workingDir = riverMobileDir
    commandLine("npm", "run", "build")
    inputs.file(riverMobileDir.resolve("package.json"))
    inputs.file(riverMobileDir.resolve("vite.config.ts"))
    inputs.file(riverMobileDir.resolve("index.html"))
    inputs.file(riverMobileDir.resolve("tsconfig.json"))
    inputs.file(riverMobileDir.resolve("tsconfig.app.json"))
    inputs.dir(riverMobileDir.resolve("src"))
    outputs.dir(riverMobileDir.resolve("dist"))
}

val riverMobileSyncAssets = tasks.register<Sync>("riverMobileSyncAssets") {
    group = "river-mobile"
    description = "Copy river-mobile dist/ into the APK's generated assets."
    dependsOn(riverMobileBuild)
    from(riverMobileDir.resolve("dist"))
    into(webappGeneratedDir)
}

tasks.named("preBuild") {
    dependsOn(riverMobileSyncAssets)
}
