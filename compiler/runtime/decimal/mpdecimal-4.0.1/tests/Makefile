
# ==============================================================================
#                        Unix Makefile for libmpdec tests
# ==============================================================================

SRCDIR = ../libmpdec
ENABLE_STATIC = yes
ENABLE_SHARED = yes

LIBSTATIC = libmpdec.a
LIBSHARED = libmpdec.4.0.1.dylib
LINK_STATIC = 
LINK_DYNAMIC = 

CC = gcc
AR = ar
MPD_GNU99 = 

FILTER_FOR_STATIC = -flto% -ffat-lto-objects

CONFIGURE_CFLAGS = -Wall -Wextra -Wno-unknown-pragmas -std=c99 -pedantic -DNDEBUG -O2 
MPD_CFLAGS_SHARED = $(strip $(filter-out $(CFLAGS),$(CONFIGURE_CFLAGS)) $(CFLAGS) $(MPD_GNU99))
MPD_CFLAGS = $(strip $(filter-out $(FILTER_FOR_STATIC),$(MPD_CFLAGS_SHARED)))

LINK_LIBSTATIC = $(strip $(LINK_STATIC) $(SRCDIR)/$(LIBSTATIC) $(LINK_DYNAMIC))


MPD_TARGETS =
ifeq ($(ENABLE_STATIC), yes)
MPD_TARGETS += runtest
endif
ifeq ($(ENABLE_SHARED), yes)
MPD_TARGETS += runtest_shared
endif

default: $(MPD_TARGETS)


# Short test.
runtest:\
Makefile runtest.c test.c $(SRCDIR)/$(LIBSTATIC) $(SRCDIR)/mpdecimal.h \
test.h vctest.h
	$(CC) -I$(SRCDIR) $(MPD_CFLAGS) -o runtest runtest.c test.c $(LINK_LIBSTATIC) -lm

runtest_shared:\
Makefile runtest.c test.c $(SRCDIR)/$(LIBSHARED) $(SRCDIR)/mpdecimal.h \
test.h vctest.h
	$(CC) -I$(SRCDIR) $(MPD_CFLAGS_SHARED) -o runtest_shared runtest.c test.c -L$(SRCDIR) -lmpdec -lm


FORCE:

clean: FORCE
	rm -f *.o *.gch *.gcda *.gcno *.gcov *.dyn *.dpi *.lock
	rm -f runtest runtest_shared

distclean: FORCE
	$(MAKE) clean
	rm -rf Makefile dectest.zip testdata
