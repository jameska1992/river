# Add project-specific ProGuard rules here.
# Keep WebView JavascriptInterface symbols if we ever add a bridge.
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}
