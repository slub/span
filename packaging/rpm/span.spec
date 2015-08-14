Summary:    Library data conversions.
Name:       span
Version:    0.1.51
Release:    0
License:    MIT
BuildArch:  x86_64
BuildRoot:  %{_tmppath}/%{name}-build
Group:      System/Base
Vendor:     Leipzig University Library, https://www.ub.uni-leipzig.de
URL:        https://github.com/miku/span

%description

Library data conversions.

%prep

%build

%pre
PATH=$PATH:/usr/local/bin:/usr/local/sbin

type taskhome > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "SPAN: Purging obsolete data artifacts..."
    rm -vrf $(taskhome)/028/DOAJIntermediateSchema
    rm -vrf $(taskhome)/048/GBIIntermediateSchema
    rm -vrf $(taskhome)/ai/AIExport
    rm -vrf $(taskhome)/ai/AIIntermediateSchema
    rm -vrf $(taskhome)/crossref/CrossrefIntermediateSchema
    rm -vrf $(taskhome)/degruyter/DegruyterIntermediateSchema
    rm -vrf $(taskhome)/jstor/JstorIntermediateSchema
else
    echo "SPAN: Nothing to do for pre or siskin not installed and configured."
fi


%install
mkdir -p $RPM_BUILD_ROOT/usr/local/sbin
install -m 755 span-export $RPM_BUILD_ROOT/usr/local/sbin
install -m 755 span-gh-dump $RPM_BUILD_ROOT/usr/local/sbin
install -m 755 span-import $RPM_BUILD_ROOT/usr/local/sbin

%post

%clean
rm -rf $RPM_BUILD_ROOT
rm -rf %{_tmppath}/%{name}
rm -rf %{_topdir}/BUILD/%{name}

%files
%defattr(-,root,root)

/usr/local/sbin/span-export
/usr/local/sbin/span-gh-dump
/usr/local/sbin/span-import

%changelog
* Fri Aug 14 2015 Martin Czygan
- 0.1.51 release
- no new features, just internal refactoring
- XML and JSON sources are now simpler to get started with FromJSON, FromXML
- slight performance gains

* Tue Aug 11 2015 Martin Czygan
- 0.1.50 release
- use a pre-script to purge affected artifacts

* Sat Aug 1 2015 Martin Czygan
- 0.1.48 release
- add -doi-blacklist flag

* Mon Jul 6 2015 Martin Czygan
- 0.1.41 release
- much faster language detection with cld2 (libc sensible)

* Sat Jun 6 2015 Martin Czygan
- 0.1.36 release
- add genios/gbi support

* Mon Jun 1 2015 Martin Czygan
- 0.1.35 release
- initial support for multiple exporters

* Sun Mar 15 2015 Martin Czygan
- 0.1.11 release
- added intermediate schema to the repo

* Thu Feb 19 2015 Martin Czygan
- 0.1.8 release
- import/export

* Thu Feb 19 2015 Martin Czygan
- 0.1.7 release
- first appearance of an intermediate format

* Wed Feb 11 2015 Martin Czygan
- 0.1.2 release
- initial release
