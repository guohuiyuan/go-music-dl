//go:build android

package main

import (
	"log"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

const (
	androidR          = 30
	storageRequestID  = 1001
	permissionGranted = 0

	readExternalStorage  = "android.permission.READ_EXTERNAL_STORAGE"
	writeExternalStorage = "android.permission.WRITE_EXTERNAL_STORAGE"
)

func (a *desktopApp) requestStoragePermission(evt app.ViewEvent) {
	if a.storagePermissionOnce {
		return
	}
	androidEvt, ok := evt.(app.AndroidViewEvent)
	if !ok || androidEvt.View == 0 {
		return
	}

	a.storagePermissionOnce = true
	view := jni.Object(androidEvt.View)
	go a.window.Run(func() {
		if err := requestStoragePermissionFromView(view); err != nil {
			log.Printf("request storage permission: %v", err)
		}
	})
}

func requestStoragePermissionFromView(view jni.Object) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		activity, err := activityFromView(env, view)
		if err != nil {
			return err
		}

		if androidSDK(env) >= androidR {
			return requestAllFilesAccess(env, activity)
		}
		return requestLegacyStorage(env, activity)
	})
}

func activityFromView(env jni.Env, view jni.Object) (jni.Object, error) {
	viewClass := jni.GetObjectClass(env, view)
	return jni.CallObjectMethod(env, view, jni.GetMethodID(env, viewClass, "getContext", "()Landroid/content/Context;"))
}

func androidSDK(env jni.Env) int32 {
	version := jni.FindClass(env, "android/os/Build$VERSION")
	return jni.GetStaticIntField(env, version, jni.GetStaticFieldID(env, version, "SDK_INT", "I"))
}

func requestAllFilesAccess(env jni.Env, activity jni.Object) error {
	environment := jni.FindClass(env, "android/os/Environment")
	isManager := jni.GetStaticMethodID(env, environment, "isExternalStorageManager", "()Z")
	ok, err := jni.CallStaticBooleanMethod(env, environment, isManager)
	if err != nil || ok {
		return err
	}

	intentClass := jni.FindClass(env, "android/content/Intent")
	intent, err := jni.NewObject(env, intentClass, jni.GetMethodID(env, intentClass, "<init>", "()V"))
	if err != nil {
		return err
	}
	if _, err := jni.CallObjectMethod(env, intent, jni.GetMethodID(env, intentClass, "setAction", "(Ljava/lang/String;)Landroid/content/Intent;"), jni.Value(jni.JavaString(env, "android.settings.MANAGE_APP_ALL_FILES_ACCESS_PERMISSION"))); err != nil {
		return err
	}

	uriClass := jni.FindClass(env, "android/net/Uri")
	packageName, err := jni.CallObjectMethod(env, activity, jni.GetMethodID(env, jni.GetObjectClass(env, activity), "getPackageName", "()Ljava/lang/String;"))
	if err != nil {
		return err
	}
	uri, err := jni.CallStaticObjectMethod(
		env,
		uriClass,
		jni.GetStaticMethodID(env, uriClass, "fromParts", "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)Landroid/net/Uri;"),
		jni.Value(jni.JavaString(env, "package")),
		jni.Value(packageName),
		0,
	)
	if err != nil {
		return err
	}
	if _, err := jni.CallObjectMethod(env, intent, jni.GetMethodID(env, intentClass, "setData", "(Landroid/net/Uri;)Landroid/content/Intent;"), jni.Value(uri)); err != nil {
		return err
	}
	return startActivity(env, activity, intent)
}

func requestLegacyStorage(env jni.Env, activity jni.Object) error {
	if androidSDK(env) < 23 || hasPermission(env, activity, readExternalStorage) && hasPermission(env, activity, writeExternalStorage) {
		return nil
	}

	stringClass := jni.FindClass(env, "java/lang/String")
	permissions := jni.NewObjectArray(env, 2, stringClass, 0)
	if err := jni.SetObjectArrayElement(env, permissions, 0, jni.Object(jni.JavaString(env, readExternalStorage))); err != nil {
		return err
	}
	if err := jni.SetObjectArrayElement(env, permissions, 1, jni.Object(jni.JavaString(env, writeExternalStorage))); err != nil {
		return err
	}
	activityClass := jni.GetObjectClass(env, activity)
	return jni.CallVoidMethod(
		env,
		activity,
		jni.GetMethodID(env, activityClass, "requestPermissions", "([Ljava/lang/String;I)V"),
		jni.Value(permissions),
		jni.Value(storageRequestID),
	)
}

func hasPermission(env jni.Env, activity jni.Object, permission string) bool {
	activityClass := jni.GetObjectClass(env, activity)
	result, err := jni.CallIntMethod(
		env,
		activity,
		jni.GetMethodID(env, activityClass, "checkSelfPermission", "(Ljava/lang/String;)I"),
		jni.Value(jni.JavaString(env, permission)),
	)
	return err == nil && result == permissionGranted
}

func startActivity(env jni.Env, activity, intent jni.Object) error {
	activityClass := jni.GetObjectClass(env, activity)
	return jni.CallVoidMethod(
		env,
		activity,
		jni.GetMethodID(env, activityClass, "startActivity", "(Landroid/content/Intent;)V"),
		jni.Value(intent),
	)
}
