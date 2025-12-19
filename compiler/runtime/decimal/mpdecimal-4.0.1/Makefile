
# ==============================================================================
#                      Unix Makefile for libmpdec/libmpdec++
# ==============================================================================

PACKAGE_TARNAME = mpdecimal
INSTALL = /opt/homebrew/bin/ginstall -c

ENABLE_CXX = yes
ENABLE_STATIC = yes
ENABLE_SHARED = yes
ENABLE_PC = yes
ENABLE_DOC = yes
ENABLE_MINGW = no
PROFILE =

LIBSTATIC = libmpdec.a
LIBNAME = libmpdec.dylib
LIBSONAME = libmpdec.4.dylib
LIBSHARED = libmpdec.4.0.1.dylib
LIBIMPORT = 

LIBSTATIC_CXX = libmpdec++.a
LIBNAME_CXX = libmpdec++.dylib
LIBSONAME_CXX = libmpdec++.4.dylib
LIBSHARED_CXX = libmpdec++.4.0.1.dylib
LIBIMPORT_CXX = 

LIBSHARED_USE_AR = no

srcdir = .
prefix = /Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/runtime/decimal/mpdecimal-4.0.1/..
exec_prefix = ${prefix}
bindir = ${exec_prefix}/bin
includedir = ${prefix}/include
libdir = ${exec_prefix}/lib
datarootdir = ${prefix}/share
docdir = ${datarootdir}/doc/${PACKAGE_TARNAME}
mandir = ${datarootdir}/man


ifeq ($(ENABLE_CXX), yes)
default: libcxx

check:
	cd libmpdec && $(MAKE) check
	cd libmpdec++ && $(MAKE) check

check_local:
	cd libmpdec && $(MAKE) check_local
	cd libmpdec++ && $(MAKE) check_local

check_alloc:
	cd libmpdec && $(MAKE) check_alloc
	cd libmpdec++ && $(MAKE) check_alloc
else
default: lib

check:
	cd libmpdec && $(MAKE) check

check_local:
	cd libmpdec && $(MAKE) check_local

check_alloc:
	cd libmpdec && $(MAKE) check_alloc
endif


lib:
	cd libmpdec && $(MAKE) $(PROFILE)

libcxx: lib
	cd libmpdec++ && $(MAKE) $(PROFILE)


install: install_dirs install_files

install_dirs: default
ifeq ($(ENABLE_MINGW), yes)
	$(INSTALL) -d -m 755 $(DESTDIR)$(bindir)
endif
	$(INSTALL) -d -m 755 $(DESTDIR)$(includedir)
	$(INSTALL) -d -m 755 $(DESTDIR)$(libdir)
	$(INSTALL) -d -m 755 $(DESTDIR)$(docdir)
ifeq ($(ENABLE_PC), yes)
	$(INSTALL) -d -m 755 $(DESTDIR)$(libdir)/pkgconfig
endif
ifeq ($(ENABLE_DOC), yes)
	$(INSTALL) -d -m 755 $(DESTDIR)$(mandir)/man3
endif

install_files: install_dirs
	$(INSTALL) -m 644 libmpdec/mpdecimal.h $(DESTDIR)$(includedir)
ifeq ($(ENABLE_STATIC), yes)
	$(INSTALL) -m 644 libmpdec/$(LIBSTATIC) $(DESTDIR)$(libdir)
endif
ifeq ($(ENABLE_SHARED), yes)
ifeq ($(ENABLE_MINGW), yes)
	$(INSTALL) -m 644 libmpdec/$(LIBIMPORT) $(DESTDIR)$(libdir)
	$(INSTALL) -m 755 libmpdec/$(LIBSHARED) $(DESTDIR)$(bindir)
else
ifeq ($(LIBSHARED_USE_AR), no)
	$(INSTALL) -m 755 libmpdec/$(LIBSHARED) $(DESTDIR)$(libdir)
	cd $(DESTDIR)$(libdir) && ln -sf $(LIBSHARED) $(LIBSONAME) && ln -sf $(LIBSHARED) $(LIBNAME)
endif
endif
endif

ifeq ($(ENABLE_CXX), yes)
	$(INSTALL) -m 644 libmpdec++/decimal.hh $(DESTDIR)$(includedir)
ifeq ($(ENABLE_STATIC), yes)
	$(INSTALL) -m 644 libmpdec++/$(LIBSTATIC_CXX) $(DESTDIR)$(libdir)
endif
ifeq ($(ENABLE_SHARED), yes)
ifeq ($(ENABLE_MINGW), yes)
	$(INSTALL) -m 644 libmpdec++/$(LIBIMPORT_CXX) $(DESTDIR)$(libdir)
	$(INSTALL) -m 755 libmpdec++/$(LIBSHARED_CXX) $(DESTDIR)$(bindir)
else
ifeq ($(LIBSHARED_USE_AR), no)
	$(INSTALL) -m 755 libmpdec++/$(LIBSHARED_CXX) $(DESTDIR)$(libdir)
	cd $(DESTDIR)$(libdir) && ln -sf $(LIBSHARED_CXX) $(LIBSONAME_CXX) && ln -sf $(LIBSHARED_CXX) $(LIBNAME_CXX)
endif
endif
endif
endif

ifeq ($(ENABLE_PC), yes)
	$(INSTALL) -m 644 libmpdec/.pc/libmpdec.pc $(DESTDIR)$(libdir)/pkgconfig
ifeq ($(ENABLE_CXX), yes)
	$(INSTALL) -m 644 libmpdec++/.pc/libmpdec++.pc $(DESTDIR)$(libdir)/pkgconfig
endif
endif

	$(INSTALL) -m 644 doc/COPYRIGHT.txt $(DESTDIR)$(docdir)
ifeq ($(ENABLE_DOC), yes)
	$(INSTALL) -m 644 doc/mpdecimal.3 $(DESTDIR)$(mandir)/man3
	$(INSTALL) -m 644 doc/libmpdec.3 $(DESTDIR)$(mandir)/man3
ifeq ($(ENABLE_CXX), yes)
	$(INSTALL) -m 644 doc/libmpdec++.3 $(DESTDIR)$(mandir)/man3
endif
endif


profile: PROFILE := profile
profile: default


clean:
	cd libmpdec && if [ -f Makefile ]; then $(MAKE) clean; else exit 0; fi
	cd libmpdec++ && if [ -f Makefile ]; then $(MAKE) clean; else exit 0; fi
	cd tests && if [ -f Makefile ]; then $(MAKE) clean; else exit 0; fi
	cd tests++ && if [ -f Makefile ]; then $(MAKE) clean; else exit 0; fi

distclean:
	cd libmpdec && if [ -f Makefile ]; then $(MAKE) distclean; else exit 0; fi
	cd libmpdec++ && if [ -f Makefile ]; then $(MAKE) distclean; else exit 0; fi
	cd tests && if [ -f Makefile ]; then $(MAKE) distclean; else exit 0; fi
	cd tests++ && if [ -f Makefile ]; then $(MAKE) distclean; else exit 0; fi
	rm -f config.h config.log config.status Makefile
	rm -rf autom4te.cache
