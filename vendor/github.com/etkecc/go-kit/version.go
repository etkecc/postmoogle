package kit

import "runtime/debug"

// Version answers "what version am I?" from the running binary's build info, whether the caller is
// the app itself (the main module, or pass "") or a library down in the dep tree. First hit wins:
//
//	first non-empty override > Main.Version > Deps[module] > VCS revision(+"-dirty") > fallback
//
// override carries an app's -X ldflag value; "(devel)" and "" both count as unstamped. The VCS
// revision is the main module's own repo, so it stays out of the dep path on purpose: a library
// reports v0.0.0 in dev, never the host binary's git sha, which would be a confident lie about
// whose code is running. Reads build info every call; wrap in sync.OnceValue on a hot path.
func Version(module, fallback string, override ...string) string {
	if len(override) > 0 && override[0] != "" {
		return override[0]
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallback
	}

	var v string
	if module == "" || info.Main.Path == module {
		v = mainVersion(info)
	} else {
		v = depVersion(info, module)
	}
	if v != "" {
		return v
	}
	return fallback
}

// mainVersion is the app's own version: the stamped release, else the git revision it was built from.
func mainVersion(info *debug.BuildInfo) string {
	if v := cleanVersion(info.Main.Version); v != "" {
		return v
	}
	return vcsVersion(info)
}

// depVersion is a dependency's own version, hopping through a replace directive: under a local replace
// dep.Version is the stale pin, not the fork you're actually compiling.
func depVersion(info *debug.BuildInfo, module string) string {
	for _, dep := range info.Deps {
		if dep.Path != module {
			continue
		}
		if dep.Replace != nil {
			dep = dep.Replace
		}
		return cleanVersion(dep.Version)
	}
	return ""
}

// UserAgent builds a "product/version" User-Agent, version resolved via Version with a v0.0.0 fallback.
// A library passes its own module path; an app passes "" (or its path) plus an optional ldflag override.
func UserAgent(product, module string, override ...string) string {
	return product + "/" + Version(module, "v0.0.0", override...)
}

// cleanVersion drops the "(devel)" placeholder so an unstamped build falls through to the next source.
func cleanVersion(v string) string {
	if v == "(devel)" {
		return ""
	}
	return v
}

// vcsVersion is the git "revision(-dirty)" the binary was built from, for when no release version was stamped.
func vcsVersion(info *debug.BuildInfo) string {
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
			if len(rev) > 7 {
				rev = rev[:7]
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev == "" {
		return ""
	}
	return rev + dirty
}
