Name:           sword-tui
Version:        2.0.0
Release:        1%{?dist}
Summary:        Terminal Bible reader built with Go and Bubbletea

License:        GPL-2.0-or-later
URL:            https://github.com/kmf/sword-tui
Source0:        %{url}/archive/v%{version}/%{name}-%{version}.tar.gz

# Builds offline-fetching the network for Go modules, which is the
# standard workflow for COPR. Disable network-isolated builds for now.
%global debug_package %{nil}

BuildRequires:  golang >= 1.21
BuildRequires:  go-rpm-macros
# Tests are visual (TUI); no automated test suite, so no Check stage.
Requires:       glibc

ExclusiveArch:  %{go_arches}

%description
sword-tui is a multi-pane terminal Bible reader written in Go. It uses
the bolls.life Bible API for translations, supports offline caching,
side-by-side comparison across translations, themeable colorschemes
(Catppuccin, Dracula, Rosé Pine, Solarized, Bru, Jozi City Nights),
verse highlighting, click+drag verse range selection, and mouse hover
indicators.


%prep
%autosetup -n %{name}-%{version}


%build
# Match the PKGBUILD's flags: PIE, trimpath, external linker, modcacherw.
export CGO_CPPFLAGS="%{?build_cppflags}"
export CGO_CFLAGS="%{?build_cflags}"
export CGO_CXXFLAGS="%{?build_cxxflags}"
export CGO_LDFLAGS="%{?build_ldflags}"
export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw -ldflags=-linkmode=external"

go build -o %{name} ./cmd/%{name}


%install
install -Dm0755 %{name} %{buildroot}%{_bindir}/%{name}
install -Dm0644 README.md %{buildroot}%{_docdir}/%{name}/README.md
install -Dm0644 CONTRIBUTING.md %{buildroot}%{_docdir}/%{name}/CONTRIBUTING.md
install -Dm0644 LICENSE %{buildroot}%{_licensedir}/%{name}/LICENSE


%files
%license LICENSE
%doc README.md CONTRIBUTING.md
%{_bindir}/%{name}


%changelog
* Sat Jun 06 2026 Karl Fischer <kmf@fischer.org.za> - 2.0.0-1
- Major release: multi-pane shell on charm v2 stack
- New: permanent two-pane layout with rounded borders and drop-shadow overlays
- New: mouse support (click, drag, hover, wheel) for navigation and verse selection
- New: 5 themes (Bru Espresso, Bru Latte, Jozi Nights/Morning/Midnight)
- New: terminal background auto-detection picks a light or dark default theme
- New: live theme preview card in the picker
- New: sticky chapter header morphs into a scroll indicator when scrolled
- New: side-by-side comparison view with per-column translation pickers
- New: real byte-level progress bar for translation downloads
- New: smart verse-reference parsing (rom8:8, rom 8 8, 1 john 3 16, etc.)
- Fix: light themes no longer leak terminal default bg into the layout

* Wed Jan 14 2026 Karl Fischer <kmf@fischer.org.za> - 1.11.0-1
- Persistence for theme and reading position
- Synchronous settings save on quit to prevent race condition
- Homebrew distribution support
