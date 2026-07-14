//go:build android

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

const bundledFFmpegAssetRoot = "ffmpeg"

func (a *desktopApp) configureBundledFFmpeg(evt app.ViewEvent) {
	if a.bundledFFmpegOnce {
		return
	}
	androidEvt, ok := evt.(app.AndroidViewEvent)
	if !ok || androidEvt.View == 0 {
		return
	}

	a.bundledFFmpegOnce = true
	view := jni.Object(androidEvt.View)
	go a.window.Run(func() {
		if err := configureBundledFFmpegFromView(view); err != nil {
			log.Printf("configure bundled ffmpeg: %v", err)
		}
	})
}

func configureBundledFFmpegFromView(view jni.Object) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		activity, err := activityFromView(env, view)
		if err != nil {
			return err
		}

		// Android 10+ (targetSdk >= 29) only permits executing binaries from the
		// extracted native library dir. ffmpeg/ffprobe ship as lib/<abi>/lib*.so,
		// which PackageManager extracts to nativeLibraryDir at install time, so
		// prefer that path.
		nativeLibraryDir, nativeErr := nativeLibraryDirFromContext(env, activity)
		if nativeErr == nil {
			if err := configureBundledFFmpegFromNativeLibraryDir(nativeLibraryDir); err == nil {
				return nil
			} else {
				log.Printf("configure bundled ffmpeg from native library dir: %v", err)
			}
		} else {
			log.Printf("resolve native library dir: %v", nativeErr)
		}

		// Fallback for APKs built by older scripts that placed the executables
		// under assets/ffmpeg/<abi>/ (only works on Android < 10).
		filesDir, err := filesDirFromContext(env, activity)
		if err != nil {
			return err
		}
		abi := androidABIForRuntime()
		if abi == "" {
			return fmt.Errorf("unsupported android abi for %s", runtime.GOARCH)
		}
		return extractBundledFFmpegFromAssets(env, activity, filesDir, abi)
	})
}

func nativeLibraryDirFromContext(env jni.Env, context jni.Object) (string, error) {
	contextClass := jni.GetObjectClass(env, context)
	applicationInfo, err := jni.CallObjectMethod(
		env,
		context,
		jni.GetMethodID(env, contextClass, "getApplicationInfo", "()Landroid/content/pm/ApplicationInfo;"),
	)
	if err != nil {
		return "", err
	}

	applicationInfoClass := jni.GetObjectClass(env, applicationInfo)
	nativeLibraryDir := jni.GetObjectField(
		env,
		applicationInfo,
		jni.GetFieldID(env, applicationInfoClass, "nativeLibraryDir", "Ljava/lang/String;"),
	)
	return jni.GoString(env, jni.String(nativeLibraryDir)), nil
}

func filesDirFromContext(env jni.Env, context jni.Object) (string, error) {
	contextClass := jni.GetObjectClass(env, context)
	filesDir, err := jni.CallObjectMethod(
		env,
		context,
		jni.GetMethodID(env, contextClass, "getFilesDir", "()Ljava/io/File;"),
	)
	if err != nil {
		return "", err
	}

	fileClass := jni.GetObjectClass(env, filesDir)
	path, err := jni.CallObjectMethod(
		env,
		filesDir,
		jni.GetMethodID(env, fileClass, "getAbsolutePath", "()Ljava/lang/String;"),
	)
	if err != nil {
		return "", err
	}
	return jni.GoString(env, jni.String(path)), nil
}

func androidABIForRuntime() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64-v8a"
	case "arm":
		return "armeabi-v7a"
	case "amd64":
		return "x86_64"
	case "386":
		return "x86"
	default:
		return ""
	}
}

func extractBundledFFmpegFromAssets(env jni.Env, context jni.Object, filesDir, abi string) error {
	filesDir = filepath.Clean(filesDir)
	if filesDir == "" || filesDir == "." {
		return fmt.Errorf("empty files dir")
	}

	targetDir := filepath.Join(filesDir, bundledFFmpegAssetRoot, abi)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	assetManager, err := assetManagerFromContext(env, context)
	if err != nil {
		return err
	}

	for _, tool := range []string{"ffmpeg", "ffprobe", "libc++_shared.so"} {
		assetName := fmt.Sprintf("%s/%s/%s", bundledFFmpegAssetRoot, abi, tool)
		targetPath := filepath.Join(targetDir, tool)
		if err := extractAssetToFile(env, assetManager, assetName, targetPath); err != nil {
			return err
		}
	}
	return configureBundledFFmpegFromExtractDir(targetDir)
}

func assetManagerFromContext(env jni.Env, context jni.Object) (jni.Object, error) {
	contextClass := jni.GetObjectClass(env, context)
	return jni.CallObjectMethod(
		env,
		context,
		jni.GetMethodID(env, contextClass, "getAssets", "()Landroid/content/res/AssetManager;"),
	)
}

func extractAssetToFile(env jni.Env, assetManager jni.Object, assetName, targetPath string) error {
	input, err := openAsset(env, assetManager, assetName)
	if err != nil {
		return err
	}
	defer closeJavaObject(env, input)

	if shouldKeepExistingBundledTool(env, input, targetPath) {
		return os.Chmod(targetPath, 0755)
	}

	tmpPath := targetPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	writeOK := false
	defer func() {
		_ = out.Close()
		if !writeOK {
			_ = os.Remove(tmpPath)
		}
	}()

	inputClass := jni.GetObjectClass(env, input)
	readMethod := jni.GetMethodID(env, inputClass, "read", "([B)I")
	buffer := jni.NewByteArray(env, make([]byte, 32*1024))
	defer jni.DeleteLocalRef(env, jni.Object(buffer))

	for {
		n, err := jni.CallIntMethod(env, input, readMethod, jni.Value(buffer))
		if err != nil {
			return err
		}
		if n < 0 {
			break
		}
		if n == 0 {
			continue
		}
		chunk := jni.GetByteArrayElements(env, buffer)
		if _, err := out.Write(chunk[:n]); err != nil {
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return err
	}
	writeOK = true
	return nil
}

func openAsset(env jni.Env, assetManager jni.Object, assetName string) (jni.Object, error) {
	assetManagerClass := jni.GetObjectClass(env, assetManager)
	return jni.CallObjectMethod(
		env,
		assetManager,
		jni.GetMethodID(env, assetManagerClass, "open", "(Ljava/lang/String;)Ljava/io/InputStream;"),
		jni.Value(jni.JavaString(env, assetName)),
	)
}

func shouldKeepExistingBundledTool(env jni.Env, input jni.Object, targetPath string) bool {
	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		return false
	}
	inputClass := jni.GetObjectClass(env, input)
	available, err := jni.CallIntMethod(env, input, jni.GetMethodID(env, inputClass, "available", "()I"))
	if err != nil || available <= 0 {
		return false
	}
	return info.Size() == int64(available)
}

func closeJavaObject(env jni.Env, obj jni.Object) {
	if obj == 0 {
		return
	}
	objClass := jni.GetObjectClass(env, obj)
	_ = jni.CallVoidMethod(env, obj, jni.GetMethodID(env, objClass, "close", "()V"))
}
