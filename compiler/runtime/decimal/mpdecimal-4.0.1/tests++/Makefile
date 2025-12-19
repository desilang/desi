
# ==============================================================================
#                       Unix Makefile for libmpdec++ tests
# ==============================================================================

SRCDIR = ../libmpdec

ENABLE_STATIC = yes
ENABLE_SHARED = yes

LIBSTATIC = libmpdec.a
LIBSHARED = libmpdec.4.0.1.dylib
LINK_STATIC = 
LINK_DYNAMIC = 

SRCDIR_CXX = ../libmpdec++
LIBSTATIC_CXX = libmpdec++.a
LIBSHARED_CXX = libmpdec++.4.0.1.dylib

LIBSHARED_USE_AR = no

CXX = g++
MPD_PTHREAD = -pthread
MPD_CXX = $(strip $(CXX) $(MPD_PTHREAD))

FILTER_FOR_STATIC = -flto% -ffat-lto-objects

CONFIGURE_CXXFLAGS = -Wall -Wextra -std=c++11 -pedantic  -DNDEBUG -O3 
MPD_CXXFLAGS_SHARED = $(strip $(filter-out $(CXXFLAGS),$(CONFIGURE_CXXFLAGS)) $(CXXFLAGS))
MPD_CXXFLAGS = $(strip $(filter-out $(FILTER_FOR_STATIC),$(MPD_CXXFLAGS_SHARED)))

LINK_LIBSTATIC = $(strip $(LINK_STATIC) $(SRCDIR_CXX)/$(LIBSTATIC_CXX) $(SRCDIR)/$(LIBSTATIC) $(LINK_DYNAMIC))


TEST_LIBS = $(SRCDIR)/$(LIBSTATIC) $(SRCDIR_CXX)/$(LIBSTATIC_CXX)
ifeq ($(LIBSHARED_USE_AR), yes)
TEST_SHLIBS = $(SRCDIR)/$(LIBSHARED) $(SRCDIR_CXX)/$(LIBSTATIC_CXX)
else
TEST_SHLIBS = $(SRCDIR)/$(LIBSHARED) $(SRCDIR_CXX)/$(LIBSHARED_CXX)
endif


MPD_TARGETS =
ifeq ($(ENABLE_STATIC), yes)
MPD_TARGETS += runtest apitest
endif

ifeq ($(ENABLE_SHARED), yes)
MPD_TARGETS += runtest_shared apitest_shared
endif


default: $(MPD_TARGETS)


# Short test:
RUNTEST_SOURCES = runtest.cc test.cc
RUNTEST_HEADERS = test.hh vctest.hh ../config.h $(SRCDIR)/mpdecimal.h $(SRCDIR_CXX)/decimal.hh

runtest:\
Makefile $(RUNTEST_SOURCES) $(RUNTEST_HEADERS) $(TEST_LIBS)
	$(MPD_CXX) -I.. -I$(SRCDIR) -I$(SRCDIR_CXX) $(MPD_CXXFLAGS) -o runtest runtest.cc test.cc $(LINK_LIBSTATIC) -lm

runtest_shared:\
Makefile $(RUNTEST_SOURCES) $(RUNTEST_HEADERS) $(TEST_SHLIBS)
	$(MPD_CXX) -I.. -I$(SRCDIR) -I$(SRCDIR_CXX) $(MPD_CXXFLAGS_SHARED) -o runtest_shared runtest.cc test.cc -L$(SRCDIR) -L$(SRCDIR_CXX) -lmpdec++ -lmpdec -lm


# API test:
APITEST_SOURCES = apitest.cc test.cc
APITEST_HEADERS = test.hh vctest.hh $(SRCDIR)/mpdecimal.h $(SRCDIR_CXX)/decimal.hh

apitest:\
Makefile $(APITEST_SOURCES) $(APITEST_HEADERS) $(TEST_LIBS)
	$(MPD_CXX) -I$(SRCDIR) -I$(SRCDIR_CXX) $(MPD_CXXFLAGS) -o apitest apitest.cc test.cc $(LINK_LIBSTATIC) -lm

apitest_shared:\
Makefile $(APITEST_SOURCES) $(APITEST_HEADERS) $(TEST_SHLIBS)
	$(MPD_CXX) -I$(SRCDIR) -I$(SRCDIR_CXX) $(MPD_CXXFLAGS_SHARED) -o apitest_shared apitest.cc test.cc -L$(SRCDIR) -L$(SRCDIR_CXX) -lmpdec++ -lmpdec -lm


FORCE:

clean: FORCE
	rm -f *.o *.gch *.gcda *.gcno *.gcov *.dyn *.dpi *.lock
	rm -f runtest runtest_shared apitest apitest_shared

distclean: FORCE
	$(MAKE) clean
	rm -rf Makefile dectest.zip testdata
