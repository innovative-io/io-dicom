#ifndef OSCONFIG_H
#define OSCONFIG_H

/*
** Define enclosures for include files with C linkage (mostly system headers)
*/
#ifdef __cplusplus
#define BEGIN_EXTERN_C extern "C" {
#define END_EXTERN_C }
#else
#define BEGIN_EXTERN_C
#define END_EXTERN_C
#endif

/* Define when building a shared library */
/* #undef DCMTK_SHARED */

/* Define when building the whole toolkit as a single shared library */
/* #undef DCMTK_BUILD_SINGLE_SHARED_LIBRARY */

/* Define when the compiler supports hidden visibility */
/* #undef HAVE_HIDDEN_VISIBILITY */

/* Define when building with wide char file I/O functions */
/* #undef WIDE_CHAR_FILE_IO_FUNCTIONS */

/* Define when building command line tools with wide char main function */
/* #undef WIDE_CHAR_MAIN_FUNCTION */

#ifdef _WIN32
  /* Define if you have the <windows.h> header file. */
/* #undef HAVE_WINDOWS_H */
  /* Define if you have the <winsock.h> header file. */
/* #undef HAVE_WINSOCK_H */
#endif

/* Define the canonical host system type as a string constant */
#define CANONICAL_HOST_TYPE "ARM_64-Darwin"

/* Define if char is unsigned on the C compiler */
/* #undef C_CHAR_UNSIGNED */

/* Define to the inline keyword supported by the C compiler, if any, or to the
   empty string */
#define C_INLINE __inline

/* Define if >> is unsigned on the C compiler */
/* #undef C_RIGHTSHIFT_UNSIGNED */

/* Define the DCMTK default path */
#define DCMTK_PREFIX "/usr/local"

/* Define the default data dictionary path for the dcmdata library package */
#define DCM_DICT_DEFAULT_PATH "/usr/local/share/dcmtk/dicom.dic"

/* Define if we want a populated builtin dictionary */
/* #undef ENABLE_BUILTIN_DICTIONARY */

/* Define if we want load external dictionaries */
#define ENABLE_EXTERNAL_DICTIONARY "1"

/* Define the environment variable path separator */
#define ENVIRONMENT_PATH_SEPARATOR ':'

/* Define to 1 if you have the `accept' function. */
#define HAVE_ACCEPT 1

/* Define to 1 if you have the `access' function. */
#define HAVE_ACCESS 1

/* Define to 1 if you have the <alloca.h> header file. */
#define HAVE_ALLOCA_H 1

/* Define to 1 if you have the <arpa/inet.h> header file. */
#define HAVE_ARPA_INET_H 1

/* Define to 1 if you have the <assert.h> header file. */
#define HAVE_ASSERT_H 1

/* Define to 1 if you have the `bcmp' function. */
#define HAVE_BCMP 1

/* Define to 1 if you have the `bcopy' function. */
#define HAVE_BCOPY 1

/* Define to 1 if you have the `bind' function. */
#define HAVE_BIND 1

/* Define to 1 if you have the `bzero' function. */
#define HAVE_BZERO 1

/* Define if your C++ compiler can work with class templates */
#define HAVE_CLASS_TEMPLATE 1

/* Define to 1 if you have the `connect' function. */
#define HAVE_CONNECT 1

/* define if the compiler supports const_cast<> */
#define HAVE_CONST_CAST 1

/* Define to 1 if you have the <ctype.h> header file. */
#define HAVE_CTYPE_H 1

/* Define to 1 if you have the `cuserid' function. */
#define HAVE_CUSERID 1

/* Define if volatile is a known keyword */
#define HAVE_CXX_VOLATILE 1

/* Define if "const" is supported by the C compiler */
#define HAVE_C_CONST 1

/* Define if your system has a declaration for socklen_t in sys/types.h
   sys/socket.h */
#define HAVE_DECLARATION_SOCKLEN_T 1

/* Define if your system has a declaration for std::ios_base::openmode in
   iostream.h */
/* #undef HAVE_DECLARATION_STD__IOS_BASE__OPENMODE */

/* Define if your system has a declaration for struct utimbuf in sys/types.h
   utime.h sys/utime.h */
#define HAVE_DECLARATION_STRUCT_UTIMBUF 1

/* Define to 1 if you have the <dirent.h> header file, and it defines `DIR'.*/
#define HAVE_DIRENT_H 1

/* Define to 1 if you have the `_doprnt' function. */
/* #undef HAVE_DOPRNT */

/* define if the compiler supports dynamic_cast<> */
#define HAVE_DYNAMIC_CAST 1

/* Define if your system cannot pass command line arguments into main() (e.g. Macintosh) */
/* #undef HAVE_EMPTY_ARGC_ARGV */

/* Define to 1 if you have the <errno.h> header file. */
#define HAVE_ERRNO_H 1

/* Define to 1 if <errno.h> defined ENAMETOOLONG. */
#define HAVE_ENAMETOOLONG 1

/* Define if your C++ compiler supports the explicit template specialization
   syntax */
#define HAVE_EXPLICIT_TEMPLATE_SPECIALIZATION 1

/* Define to 1 if you have the <fcntl.h> header file. */
#define HAVE_FCNTL_H 1

/* Define to 1 if you have the `finite' function. */
#define HAVE_FINITE 1

/* Define to 1 if you have the <float.h> header file. */
#define HAVE_FLOAT_H 1

/* Define to 1 if you have the `flock' function. */
#define HAVE_FLOCK 1

/* Define to 1 if you have the <fnmatch.h> header file. */
#define HAVE_FNMATCH_H 1

/* Define to 1 if you have the `fork' function. */
#define HAVE_FORK 1

/* Define to 1 if fseeko (and presumably ftello) exists and is declared. */
#define HAVE_FSEEKO 1

/* Define to 1 if you have the <fstream> header file. */
#define HAVE_FSTREAM 1

/* Define to 1 if you have the <fstream.h> header file. */
/* #undef HAVE_FSTREAM_H */

/* Define to 1 if you have the `ftime' function. */
#define HAVE_FTIME 1

/* Define if your C++ compiler can work with function templates */
#define HAVE_FUNCTION_TEMPLATE 1

/* Define to 1 if you have the `getaddrinfo' function. */
#define HAVE_GETADDRINFO 1

/* Define to 1 if you have the `getenv' function. */
#define HAVE_GETENV 1

/* Define to 1 if you have the `geteuid' function. */
#define HAVE_GETEUID 1

/* Define to 1 if you have the `getgrnam' function. */
#define HAVE_GETGRNAM 1

/* Define to 1 if you have the `gethostbyname' function. */
#define HAVE_GETHOSTBYNAME 1

/* Define to 1 if you have the `gethostbyname_r' function. */
#define HAVE_GETHOSTBYNAME_R 1

/* Define to 1 if you have the `gethostbyaddr_r' function. */
#define HAVE_GETHOSTBYADDR_R 1

/* Define to 1 if you have the `getgrnam_r' function. */
#define HAVE_GETGRNAM_R 1

/* Define to 1 if you have the `getpwnam_r' function. */
#define HAVE_GETPWNAM_R 1

/* Define to 1 if you have the `gethostid' function. */
#define HAVE_GETHOSTID 1

/* Define to 1 if you have the `gethostname' function. */
#define HAVE_GETHOSTNAME 1

/* Define to 1 if you have the `getlogin' function. */
#define HAVE_GETLOGIN 1

/* Define to 1 if you have the `getlogin_r' function. */
#define HAVE_GETLOGIN_R 1

/* Define to 1 if you have the `getpid' function. */
#define HAVE_GETPID 1

/* Define to 1 if you have the `getpwnam' function. */
#define HAVE_GETPWNAM 1

/* Define to 1 if you have the `getrusage' function. */
#define HAVE_GETRUSAGE 1

/* Define to 1 if you have the `getsockname' function. */
#define HAVE_GETSOCKNAME 1

/* Define to 1 if you have the `getsockopt' function. */
#define HAVE_GETSOCKOPT 1

/* Define to 1 if you have the `gettimeofday' function. */
#define HAVE_GETTIMEOFDAY 1

/* Define to 1 if you have the `getuid' function. */
#define HAVE_GETUID 1

/* Define to 1 if you have the `gmtime_r' function. */
#define HAVE_GMTIME_R 1

/* Define to 1 if you have the <grp.h> header file. */
#define HAVE_GRP_H 1

/* Define to 1 if you have the <ieeefp.h> header file. */
/* #undef HAVE_IEEEFP_H */

/* Define to 1 if you have the `index' function. */
#define HAVE_INDEX 1

/* Define if your system declares argument 3 of accept() as int * instead of
   size_t * or socklen_t * */
/* #undef HAVE_INTP_ACCEPT */

/* Define if your system declares argument 5 of getsockopt() as int * instead
   of size_t * or socklen_t */
/* #undef HAVE_INTP_GETSOCKOPT */

/* Define if your system declares argument 2-4 of select() as int * instead of
   struct fd_set * */
/* #undef HAVE_INTP_SELECT */

/* Define to 1 if you have the <inttypes.h> header file. */
#define HAVE_INTTYPES_H 1

/* Define to 1 if you have the <iomanip> header file. */
#define HAVE_IOMANIP 1

/* Define to 1 if you have the <iomanip.h> header file. */
/* #undef HAVE_IOMANIP_H */

/* Define to 1 if you have the <iostream> header file. */
#define HAVE_IOSTREAM 1

/* Define to 1 if you have the <iostream.h> header file. */
/* #undef HAVE_IOSTREAM_H */

/* Define if your system defines ios::nocreate in iostream.h */
/* defined below */

/* Define to 1 if you have the <io.h> header file. */
/* #undef HAVE_IO_H */

/* Define to 1 if you have the `isinf' function. */
#define HAVE_ISINF 1

/* Define to 1 if you have the `isnan' function. */
#define HAVE_ISNAN 1

/* Define to 1 if you have the <iso646.h> header file. */
#define HAVE_ISO646_H 1

/* Define to 1 if you have the `itoa' function. */
/* #undef HAVE_ITOA */

/* Define to 1 if you have the <libc.h> header file. */
/* #undef HAVE_LIBC_H */

/* Define to 1 if you have the `iostream' library (-liostream). */
/* #undef HAVE_LIBIOSTREAM */

/* Define to 1 if you have the `nsl' library (-lnsl). */
#define HAVE_LIBNSL 1

/* Define to 1 if the <libpng/png.h> header shall be used instead of <png.h>. */
/* #undef HAVE_LIBPNG_PNG_H */

/* Define to 1 if you have the `socket' library (-lsocket). */
/* #undef HAVE_LIBSOCKET */

/* Define to 1 if you have the <limits.h> header file. */
#define HAVE_LIMITS_H 1

/* Define to 1 if you have the <climits> header file. */
#define HAVE_CLIMITS 1

/* Define to 1 if you have the `listen' function. */
#define HAVE_LISTEN 1

/* Define to 1 if you have the <locale.h> header file. */
#define HAVE_LOCALE_H 1

/* Define to 1 if you have the `localtime_r' function. */
#define HAVE_LOCALTIME_R 1

/* Define to 1 if you have the `lockf' function. */
#define HAVE_LOCKF 1

/* Define to 1 if you have the `lstat' function. */
#define HAVE_LSTAT 1

/* Define to 1 if you support file names longer than 14 characters. */
#define HAVE_LONG_FILE_NAMES 1

/* Define to 1 if you have the `malloc_debug' function. */
/* #undef HAVE_MALLOC_DEBUG */

/* Define to 1 if you have the <malloc.h> header file. */
#define HAVE_MALLOC_H 1

/* Define to 1 if you have the <math.h> header file. */
#define HAVE_MATH_H 1

/* Define to 1 if you have the <cmath> header file. */
#define HAVE_CMATH 1

/* Define to 1 if you have the `mbstowcs' function. */
#define HAVE_MBSTOWCS 1

/* Define to 1 if you have the `memcmp' function. */
#define HAVE_MEMCMP 1

/* Define to 1 if you have the `memcpy' function. */
#define HAVE_MEMCPY 1

/* Define to 1 if you have the `memmove' function. */
#define HAVE_MEMMOVE 1

/* Define to 1 if you have the <memory.h> header file. */
#define HAVE_MEMORY_H 1

/* Define to 1 if you have the `memset' function. */
#define HAVE_MEMSET 1

/* Define to 1 if you have the `mkstemp' function. */
#define HAVE_MKSTEMP 1

/* Define to 1 if you have the `mktemp' function. */
#define HAVE_MKTEMP 1

/* Define to 1 if you have the <ndir.h> header file, and it defines `DIR'. */
/* #undef HAVE_NDIR_H */

/* Define to 1 if you have the <netdb.h> header file. */
#define HAVE_NETDB_H 1

/* Define to 1 if you have the <netinet/in.h> header file. */
#define HAVE_NETINET_IN_H 1

/* Define to 1 if you have the <netinet/in_systm.h> header file. */
#define HAVE_NETINET_IN_SYSTM_H 1

/* Define to 1 if you have the <netinet/tcp.h> header file. */
#define HAVE_NETINET_TCP_H 1

/* Define to 1 if you have the <new.h> header file. */
/* #undef HAVE_NEW_H */

/* Define to 1 if you have the <fenv.h> header file. */
#define HAVE_FENV_H 1

/* Define to 1 if you have the <sys/systeminfo.h> header file. */
/* #undef HAVE_SYS_SYSTEMINFO_H */

/* Define to 1 if you have the <iterator> header file. */
#define HAVE_ITERATOR_HEADER 1

/* Define to 1 if you have readdir_r */
#define HAVE_READDIR_R 1

/* Define if your system supports readdir_r with the obsolete Posix 1.c draft
   6 declaration (2 arguments) instead of the Posix 1.c declaration with 3
   arguments. */
/* #undef HAVE_OLD_READDIR_R */

/* Define if your system has a prototype for feenableexcept in fenv.h */
#define HAVE_PROTOTYPE_FEENABLEEXCEPT 1

/* Define if your system has a prototype for accept in sys/types.h
   sys/socket.h */
#define HAVE_PROTOTYPE_ACCEPT 1

/* Define if your system has a prototype for bind in sys/types.h sys/socket.h */
#define HAVE_PROTOTYPE_BIND 1

/* Define if your system has a prototype for bzero in string.h strings.h
   libc.h unistd.h stdlib.h */
#define HAVE_PROTOTYPE_BZERO 1

/* Define if your system has a prototype for connect in sys/types.h
   sys/socket.h */
#define HAVE_PROTOTYPE_CONNECT 1

/* Define if your system has a prototype for finite in math.h */
#define HAVE_PROTOTYPE_FINITE 1

/* Define to 1 if your has a prototype for `TryAcquireSRWLockShared' in windows.h (Win32 only). */
/* #undef HAVE_PROTOTYPE_TRYACQUIRESRWLOCKSHARED */

/* Define if your system has a prototype for std::finite in cmath */
/* #undef HAVE_PROTOTYPE_STD__FINITE */

/* Define if your system has a prototype for flock in sys/file.h */
#define HAVE_PROTOTYPE_FLOCK 1

/* Define if your system has a prototype for gethostbyname in libc.h unistd.h
   stdlib.h netdb.h */
#define HAVE_PROTOTYPE_GETHOSTBYNAME 1

/* Define if your system has a prototype for gethostbyname_r in libc.h unistd.h
   stdlib.h netdb.h */
#define HAVE_PROTOTYPE_GETHOSTBYNAME_R 1

/* Define if your system has a prototype for gethostbyaddr_r in libc.h unistd.h
   stdlib.h netdb.h */
#define HAVE_PROTOTYPE_GETHOSTBYADDR_R 1

/* Define if your system has a prototype for gethostid in libc.h unistd.h
   stdlib.h netdb.h */
#define HAVE_PROTOTYPE_GETHOSTID 1

/* Define if your system has a prototype for gethostname in unistd.h libc.h
   stdlib.h netdb.h */
#define HAVE_PROTOTYPE_GETHOSTNAME 1

/* Define if your system has a prototype for getsockname in sys/types.h
   sys/socket.h */
#define HAVE_PROTOTYPE_GETSOCKNAME 1

/* Define if your system has a prototype for getsockopt in sys/types.h
   sys/socket.h */
#define HAVE_PROTOTYPE_GETSOCKOPT 1

/* Define if your system has a prototype for gettimeofday in sys/time.h
   unistd.h */
#define HAVE_PROTOTYPE_GETTIMEOFDAY 1

/* Define if your system has a prototype for isinf in math.h */
#define HAVE_PROTOTYPE_ISINF 1

/* Define if your system has a prototype for isnan in math.h */
#define HAVE_PROTOTYPE_ISNAN 1

/* Define if your system has a prototype for std::isinf in cmath */
#define HAVE_PROTOTYPE_STD__ISINF 1

/* Define if your system has a prototype for std::isnan in cmath */
#define HAVE_PROTOTYPE_STD__ISNAN 1

/* Define if your system has a prototype for fpclassf in math.h */
/* #undef HAVE_PROTOTYPE__FPCLASSF */

/* Define if your system has a prototype for listen in sys/types.h
  sys/socket.h */
#define HAVE_PROTOTYPE_LISTEN 1

/* Define if your system has a prototype for mkstemp in libc.h unistd.h
   stdlib.h */
#define HAVE_PROTOTYPE_MKSTEMP 1

/* Define if your system has a prototype for mktemp in libc.h unistd.h
   stdlib.h */
#define HAVE_PROTOTYPE_MKTEMP 1

/* Define if your system has a prototype for select in sys/select.h
   sys/types.h sys/socket.h sys/time.h */
#define HAVE_PROTOTYPE_SELECT 1

/* Define if your system has a prototype for setsockopt in sys/types.h
   sys/socket.h */
#define HAVE_PROTOTYPE_SETSOCKOPT 1

/* Define if your system has a prototype for socket in sys/types.h
   sys/socket.h */
#define HAVE_PROTOTYPE_SOCKET 1

/* Define if your system has a prototype for std::vfprintf in stdarg.h */
#define HAVE_PROTOTYPE_STD__VFPRINTF 1

/* Define if your system has a prototype for std::vsnprintf in stdio.h */
#define HAVE_PROTOTYPE_STD__VSNPRINTF 1

/* Define if your system has a prototype for strcasecmp in string.h */
#define HAVE_PROTOTYPE_STRCASECMP 1

/* Define if your system has a prototype for strncasecmp in string.h */
#define HAVE_PROTOTYPE_STRNCASECMP 1

/* Define if your system has a prototype for strerror_r in string.h */
#define HAVE_PROTOTYPE_STRERROR_R 1

/* Define if your system has a prototype for gettid */
#define HAVE_SYS_GETTID 1

/* Define if your system has a prototype for usleep in libc.h unistd.h
   stdlib.h */
#define HAVE_PROTOTYPE_USLEEP 1

/* Define if your system has a prototype for wait3 in libc.h sys/wait.h
   sys/time.h sys/resource.h */
#define HAVE_PROTOTYPE_WAIT3 1

/* Define if your system has a prototype for waitpid in sys/wait.h sys/time.h
   sys/resource.h */
#define HAVE_PROTOTYPE_WAITPID 1

/* Define if your system has a prototype for _stricmp in string.h */
/* #undef HAVE_PROTOTYPE__STRICMP */

/* Define if your system has a prototype for nanosleep in time.h */
#define HAVE_PROTOTYPE_NANOSLEEP 1

/* Define to 1 if you have the <pthread.h> header file. */
#define HAVE_PTHREAD_H 1

/* Define if your system supports POSIX read/write locks */
#define HAVE_PTHREAD_RWLOCK 1

/* Define to 1 if you have the <pwd.h> header file. */
#define HAVE_PWD_H 1

/* define if the compiler supports reinterpret_cast<> */
#define HAVE_REINTERPRET_CAST 1

/* Define to 1 if you have the `rindex' function. */
#define HAVE_RINDEX 1

/* Define to 1 if you have the `select' function. */
#define HAVE_SELECT 1

/* Define to 1 if you have the <semaphore.h> header file. */
#define HAVE_SEMAPHORE_H 1

/* Define to 1 if you have the <setjmp.h> header file. */
#define HAVE_SETJMP_H 1

/* Define to 1 if you have the `setsockopt' function. */
#define HAVE_SETSOCKOPT 1

/* Define to 1 if you have the `setuid' function. */
#define HAVE_SETUID 1

/* Define to 1 if you have the <signal.h> header file. */
#define HAVE_SIGNAL_H 1

/* Define to 1 if you have the `sleep' function. */
#define HAVE_SLEEP 1

/* Define to 1 if you have the `socket' function. */
#define HAVE_SOCKET 1

/* Define to 1 if you have the <sstream> header file. */
#define HAVE_SSTREAM 1

/* Define to 1 if you have the <sstream.h> header file. */
/* #undef HAVE_SSTREAM_H */

/* Define to 1 if you have the `stat' function. */
#define HAVE_STAT 1

/* define if the compiler supports static_cast<> */
#define HAVE_STATIC_CAST 1

/* Define if your C++ compiler can work with static methods in class templates
   */
#define HAVE_STATIC_TEMPLATE_METHOD 1

/* Define to 1 if you have the <stat.h> header file. */
/* #undef HAVE_STAT_H */

/* Define to 1 if you have the <stdarg.h> header file. */
#define HAVE_STDARG_H 1

/* Define to 1 if you have the <stdbool.h> header file. */
#define HAVE_STDBOOL_H 1

/* Define to 1 if you have the <cstdarg> header file. */
#define HAVE_CSTDARG 1

/* Define to 1 if you have the <stddef.h> header file. */
#define HAVE_STDDEF_H 1

/* Define to 1 if you have the <stdint.h> header file. */
#define HAVE_STDINT_H 1

/* Define to 1 if you have the <cstdint> header file. */
#define HAVE_CSTDINT 1

/* Define to 1 if you have the <stdio.h> header file. */
#define HAVE_STDIO_H 1

/* Define to 1 if you have the <cstdio> header file. */
#define HAVE_CSTDIO 1

/* Define to 1 if you have the <stdlib.h> header file. */
#define HAVE_STDLIB_H 1

/* Define if ANSI standard C++ includes use std namespace */
/* defined below */

/* Define if the compiler supports std::nothrow */
/* defined below */

/* Define to 1 if you have the `strchr' function. */
#define HAVE_STRCHR 1

/* Define to 1 if you have the `strdup' function. */
#define HAVE_STRDUP 1

/* Define to 1 if you have the `strerror' function. */
#define HAVE_STRERROR 1

/* Define to 1 if `strerror_r' returns a char*. */
#define HAVE_CHARP_STRERROR_R 1

/* Define to 1 if you have the <streambuf.h> header file. */
/* #undef HAVE_STREAMBUF_H */

/* Define to 1 if you have the <strings.h> header file. */
#define HAVE_STRINGS_H 1

/* Define to 1 if you have the <string.h> header file. */
#define HAVE_STRING_H 1

/* Define to 1 if you have the `strlcat' function. */
/* #undef HAVE_STRLCAT */

/* Define to 1 if you have the `strlcpy' function. */
/* #undef HAVE_STRLCPY */

/* Define to 1 if you have the `strstr' function. */
#define HAVE_STRSTR 1

/* Define to 1 if you have the <strstream> header file. */
#define HAVE_STRSTREAM 1

/* Define to 1 if you have the <strstream.h> header file. */
/* #undef HAVE_STRSTREAM_H */

/* Define to 1 if you have the <strstrea.h> header file. */
/* #undef HAVE_STRSTREA_H */

/* Define to 1 if you have the `strtoul' function. */
#define HAVE_STRTOUL 1

/* Define to 1 if you have the <synch.h> header file. */
/* #undef HAVE_SYNCH_H */

/* Define if __sync_add_and_fetch is available */
#define HAVE_SYNC_ADD_AND_FETCH 1

/* Define if __sync_sub_and_fetch is available */
#define HAVE_SYNC_SUB_AND_FETCH 1

/* Define if InterlockedIncrement is available */
/* #undef HAVE_INTERLOCKED_INCREMENT */

/* Define if InterlockedDecrement is available */
/* #undef HAVE_INTERLOCKED_DECREMENT */

/* Define if passwd::pw_gecos is available */
#define HAVE_PASSWD_GECOS 1

/* Define to 1 if you have the `sysinfo' function. */
#define HAVE_SYSINFO 1

/* Define to 1 if you have the <syslog.h> header file. */
#define HAVE_SYSLOG_H 1

/* Define to 1 if you have the <sys/dir.h> header file, and it defines `DIR'.*/
#define HAVE_SYS_DIR_H 1

/* Define to 1 if you have the <sys/errno.h> header file. */
#define HAVE_SYS_ERRNO_H 1

/* Define to 1 if you have the <sys/file.h> header file. */
#define HAVE_SYS_FILE_H 1

/* Define to 1 if you have the <sys/ndir.h> header file, and it defines `DIR'.*/
/* #undef HAVE_SYS_NDIR_H */

/* Define to 1 if you have the <sys/param.h> header file. */
#define HAVE_SYS_PARAM_H 1

/* Define to 1 if you have the <sys/resource.h> header file. */
#define HAVE_SYS_RESOURCE_H 1

/* Define to 1 if you have the <sys/select.h> header file. */
#define HAVE_SYS_SELECT_H 1

/* Define to 1 if you have the <sys/socket.h> header file. */
#define HAVE_SYS_SOCKET_H 1

/* Define to 1 if you have the <sys/stat.h> header file. */
#define HAVE_SYS_STAT_H 1

/* Define to 1 if you have the <sys/syscall.h> header file. */
#define HAVE_SYS_SYSCALL_H 1

/* Define to 1 if you have the <sys/timeb.h> header file. */
#define HAVE_SYS_TIMEB_H 1

/* Define to 1 if you have the <sys/time.h> header file. */
#define HAVE_SYS_TIME_H 1

/* Define to 1 if you have the <sys/types.h> header file. */
#define HAVE_SYS_TYPES_H 1

/* Define to 1 if you have the <sys/utime.h> header file. */
/* #undef HAVE_SYS_UTIME_H */

/* Define to 1 if you have the <sys/utsname.h> header file. */
#define HAVE_SYS_UTSNAME_H 1

/* Define to 1 if you have <sys/wait.h> that is POSIX.1 compatible. */
#define HAVE_SYS_WAIT_H 1

/* Define to 1 if you have the `tempnam' function. */
#define HAVE_TEMPNAM 1

/* Define to 1 if you have the <thread.h> header file. */
/* #undef HAVE_THREAD_H */

/* Define to 1 if you have the <time.h> header file. */
#define HAVE_TIME_H 1

/* Define to 1 if you have the `tmpnam' function. */
#define HAVE_TMPNAM 1

/* define if the compiler recognizes typename */
#define HAVE_TYPENAME 1

/* Define to 1 if you have the `uname' function. */
#define HAVE_UNAME 1

/* Define to 1 if you have the <unistd.h> header file. */
#define HAVE_UNISTD_H 1

/* Define to 1 if you have the <unix.h> header file. */
/* #undef HAVE_UNIX_H */

/* Define to 1 if you have the `usleep' function. */
#define HAVE_USLEEP 1

/* Define to 1 if you have the <utime.h> header file. */
#define HAVE_UTIME_H 1

/* Define to 1 if you have the `vprintf' function. */
#define HAVE_VPRINTF 1

/* Define to 1 if you have the `_vsnprintf_s' function. */
/* #undef HAVE__VSNPRINTF_S */

/* Define to 1 if you have the `vfprintf_s' function. */
/* #undef HAVE_VFPRINTF_S */

/* Define to 1 if you have the `vsnprintf' function. */
#define HAVE_VSNPRINTF 1

/* Define to 1 if you have the `vsprintf_s' function. */
/* #undef HAVE_VSPRINTF_S */

/* Define to 1 if you have the `wait3' function. */
#define HAVE_WAIT3 1

/* Define to 1 if you have the `waitpid' function. */
#define HAVE_WAITPID 1

/* Define to 1 if you have the <wchar.h> header file. */
#define HAVE_WCHAR_H 1

/* Define to 1 if you have the `wcstombs' function. */
#define HAVE_WCSTOMBS 1

/* Define to 1 if you have the <wctype.h> header file. */
#define HAVE_WCTYPE_H 1

/* Define to 1 if you have the `_findfirst' function. */
/* #undef HAVE__FINDFIRST */

/* Define to 1 if the compiler supports __FUNCTION__. */
#define HAVE___FUNCTION___MACRO 1

/* Define to 1 if the compiler supports __PRETTY_FUNCTION__. */
#define HAVE___PRETTY_FUNCTION___MACRO 1

/* Define to 1 if the compiler supports __func__. */
#define HAVE___func___MACRO 1

/* Define to 1 if you have the `nanosleep' function. */
#define HAVE_NANOSLEEP 1

/* Define if libc.h should be treated as a C++ header */
/* #undef INCLUDE_LIBC_H_AS_CXX */

/* Define if <math.h> fails if included extern "C" */
/* #undef INCLUDE_MATH_H_AS_CXX */

/* Define to 1 if you have variable length arrays. */
#define HAVE_VLA 1

/* Define to the address where bug reports for this package should be sent. */
/* #undef PACKAGE_BUGREPORT */

/* Define to the full name of this package. */
#define PACKAGE_NAME "dcmtk"

/* Define to the full name and version of this package. */
#define PACKAGE_STRING ""

/* Define to the default configuration directory (used by some applications) */
#define DEFAULT_CONFIGURATION_DIR "/usr/local/etc/dcmtk/"

/* Define to the default support data directory (used by some applications) */
#define DEFAULT_SUPPORT_DATA_DIR "/usr/local/share/dcmtk/"

/* Define to the one symbol short name of this package. */
/* #undef PACKAGE_TARNAME */

/* Define to the date of this package. */
#define PACKAGE_DATE "DEV"

/* Define to the version of this package. */
#define PACKAGE_VERSION "3.6.5"

/* Define to the version suffix of this package. */
#define PACKAGE_VERSION_SUFFIX "+"

/* Define to the version number of this package. */
#define PACKAGE_VERSION_NUMBER 365

/* Define path separator */
#define PATH_SEPARATOR '/'

/* Define as the return type of signal handlers (`int' or `void'). */
#define RETSIGTYPE void

/* Define if signal handlers need ellipse (...) parameters */
/* #undef SIGNAL_HANDLER_WITH_ELLIPSE */

/* LFS mode constants. */
#define DCMTK_LFS 1
#define DCMTK_LFS64 2

/* Select LFS mode (defined above) that shall be used or don't define it */
#define DCMTK_ENABLE_LFS DCMTK_LFS64

/* The size of a `char', as computed by sizeof. */
#define SIZEOF_CHAR 1

/* The size of a `double', as computed by sizeof. */
#define SIZEOF_DOUBLE 8

/* The size of a `float', as computed by sizeof. */
#define SIZEOF_FLOAT 4

/* The size of a `int', as computed by sizeof. */
#define SIZEOF_INT 4

/* The size of a `long', as computed by sizeof. */
#define SIZEOF_LONG 8

/* The size of a `short', as computed by sizeof. */
#define SIZEOF_SHORT 2

/* The size of a `void *', as computed by sizeof. */
#define SIZEOF_VOID_P 8

/* The size of a `fpos_t', as computed by sizeof. */
/* #undef SIZEOF_FPOS_T */

/* The size of a `off_t', as computed by sizeof. */
/* #undef SIZEOF_OFF_T */

/* Define to 1 if you have the ANSI C header files. */
#define STDC_HEADERS 1

/* Define to 1 if your <sys/time.h> declares `struct tm'. */
/* #undef TM_IN_SYS_TIME */

/* Define if ANSI standard C++ includes are used */
#define USE_STD_CXX_INCLUDES

/* Define if we are compiling with libiconv support. */
/* #undef WITH_LIBICONV */

/* Define if the C standard library has iconv builtin. */
#define WITH_STDLIBC_ICONV

/* Define if we are compiling with ICU support. */
/* #undef WITH_ICU */

/* character set conversion constants. */
#define DCMTK_CHARSET_CONVERSION_ICU 1
#define DCMTK_CHARSET_CONVERSION_ICONV 2
#define DCMTK_CHARSET_CONVERSION_STDLIBC_ICONV 3

/* Define to select character set conversion implementation. */
#define DCMTK_ENABLE_CHARSET_CONVERSION DCMTK_CHARSET_CONVERSION_STDLIBC_ICONV

/* Define if the second argument to iconv() is const */
/* #undef LIBICONV_SECOND_ARGUMENT_CONST */

/* Try to define the iconv behavior as conversion flags */
#define DCMTK_FIXED_ICONV_CONVERSION_FLAGS AbortTranscodingOnIllegalSequence

/* Define if iconv_open() accepts "" as an argument */
#define DCMTK_STDLIBC_ICONV_HAS_DEFAULT_ENCODING 1

/* Define if we are compiling with libpng support */
/* #undef WITH_LIBPNG */

/* Define if we are compiling with libtiff support */
/* #undef WITH_LIBTIFF */

/* Define if we are compiling with libxml support */
/* #undef WITH_LIBXML */

/* Define if we are compiling with OpenJPEG support */
/* #undef WITH_OPENJPEG */

/* Define if we are compiling with OpenSSL support */
#define WITH_OPENSSL

/* Define if we are compiling for built-in private tag dictionary */
/* #undef ENABLE_PRIVATE_TAGS */

/* Define if we are compiling with sndfile support. */
/* #undef WITH_SNDFILE */

/* Define if we are compiling with libwrap (TCP wrapper) support */
/* #undef WITH_TCPWRAPPER */

/* Define if we are compiling with zlib support */
/* #undef WITH_ZLIB */

/* Define if we are compiling with any type of Multi-thread support */
#define WITH_THREADS

/* Define if pthread_t is a pointer type on your system */
/* #undef HAVE_POINTER_TYPE_PTHREAD_T */

/* Define to 1 if on AIX 3.
   System headers sometimes define this.
   We just want to avoid a redefinition error message. */
#ifndef _ALL_SOURCE
/* #undef _ALL_SOURCE */
#endif

/* Define to 1 if type `char' is unsigned and you are not using gcc. */
#ifndef __CHAR_UNSIGNED__
/* #undef __CHAR_UNSIGNED__ */
#endif

/* Define `pid_t' to `int' if <sys/types.h> does not define. */
/* #undef HAVE_NO_TYPEDEF_PID_T */
#ifdef HAVE_NO_TYPEDEF_PID_T
#define pid_t int
#endif

/* Define `size_t' to `unsigned' if <sys/types.h> does not define. */
/* #undef HAVE_NO_TYPEDEF_SIZE_T */
#ifdef HAVE_NO_TYPEDEF_SIZE_T
#define size_t unsigned
#endif

/* Define `ssize_t' to `long' if <sys/types.h> does not define. */
/* #undef HAVE_NO_TYPEDEF_SSIZE_T */
#ifdef HAVE_NO_TYPEDEF_SSIZE_T
#define ssize_t long
#endif

/* Set typedefs as needed for JasPer library */
/* #undef HAVE_UCHAR_TYPEDEF */
#ifndef HAVE_UCHAR_TYPEDEF
typedef unsigned char uchar;
#endif

#define HAVE_USHORT_TYPEDEF
#ifndef HAVE_USHORT_TYPEDEF
typedef unsigned short ushort;
#endif

#define HAVE_UINT_TYPEDEF
#ifndef HAVE_UINT_TYPEDEF
typedef unsigned int uint;
#endif

#define HAVE_ULONG_TYPEDEF
#ifndef HAVE_ULONG_TYPEDEF
typedef unsigned long ulong;
#endif

#define HAVE_LONG_LONG
#define HAVE_UNSIGNED_LONG_LONG

/* evaluated by JasPer */
/* #undef HAVE_LONGLONG */
/* #undef HAVE_ULONGLONG */

#define HAVE_INT64_T 1
#define HAVE_UINT64_T 1

 /* Additional settings for Borland C++ Builder */
#ifdef __BORLANDC__
#define _stricmp stricmp    /* _stricmp in MSVC is stricmp in Borland C++ */
#define _strnicmp strnicmp  /* _strnicmp in MSVC is strnicmp in Borland C++ */
    #pragma warn -8027      /* disable Warning W8027 "functions containing while are not expanded inline" */
    #pragma warn -8004      /* disable Warning W8004 "variable is assigned a value that is never used" */
    #pragma warn -8012      /* disable Warning W8012 "comparing signed and unsigned values" */
#ifdef WITH_THREADS
#define __MT__              /* required for _beginthreadex() API in <process.h> */
#define _MT                 /* required for _errno on BCB6 */
#endif
#define HAVE_PROTOTYPE_MKTEMP 1
#undef HAVE_SYS_UTIME_H
#define _MSC_VER 1200       /* Treat Borland C++ 5.5 as MSVC6. */
#endif /* __BORLANDC__ */

/* Platform specific settings for Visual C++
 * By default, enable ANSI standard C++ includes on Visual C++ 6 and newer
 *   _MSC_VER == 1100 on Microsoft Visual C++ 5.0
 *   _MSC_VER == 1200 on Microsoft Visual C++ 6.0
 *   _MSC_VER == 1300 on Microsoft Visual C++ 7.0
 *   _MSC_VER == 1310 on Microsoft Visual C++ 7.1
 *   _MSC_VER == 1400 on Microsoft Visual C++ 8.0
 */

#ifdef _MSC_VER
#if _MSC_VER <= 1200 /* Additional settings for VC6 and older */
/* disable warning that return type for 'identifier::operator ->' is not a UDT or reference to a UDT */
    #pragma warning( disable : 4284 )
#define HAVE_OLD_INTERLOCKEDCOMPAREEXCHANGE 1
#else
#define HAVE_VSNPRINTF 1
#endif /* _MSC_VER <= 1200 */

    #pragma warning( disable : 4251 )  /* disable warnings about needed dll-interface */
                                       /* http://www.unknownroad.com/rtfm/VisualStudio/warningC4251.html */
    #pragma warning( disable : 4099 )  /* disable warning about mismatched class and struct keywords */
                                       /* http://alfps.wordpress.com/2010/06/22/cppx-is-c4099-really-a-sillywarning-disabling-msvc-sillywarnings */
    #pragma warning( disable : 4521 )  /* disable warnings about multiple copy constructors and assignment operators,*/
    #pragma warning( disable : 4522 )  /* since these are sometimes necessary for correct overload resolution*/

#if _MSC_VER >= 1400                   /* Additional settings for Visual Studio 2005 and newer */
    #pragma warning( disable : 4996 )  /* disable warnings about "deprecated" C runtime functions */
    #pragma warning( disable : 4351 )  /* disable warnings about "new behavior" when initializing the elements of an array */
#endif /* _MSC_VER >= 1400 */
#endif /* _MSC_VER */

/* Define if your system defines ios::nocreate in iostream.h */
/* #undef HAVE_IOS_NOCREATE */

/* Define if ANSI standard C++ includes use std namespace */
#define HAVE_STD_NAMESPACE 1

/* Define if the compiler supports std::nothrow */
#define HAVE_STD__NOTHROW 1

/* Define if the compiler supports operator delete (std::nothrow) */
#define HAVE_NOTHROW_DELETE 1

/* Define if the compiler supports static_assert */
#define HAVE_STATIC_ASSERT 1

/* Define if your system has a prototype for std::vfprintf in stdarg.h */
#define HAVE_PROTOTYPE_STD__VFPRINTF 1

/* Define if your system has off64_t */
#define HAVE_OFF64_T 1

/* Define if your system has fpos64_t */
#define HAVE_FPOS64_T 1

/* Define if your system uses _popen instead of popen */
#define HAVE_POPEN 1

/* Define if your system uses _pclose instead of pclose */
#define HAVE_PCLOSE 1

/* Define if your system provides sigjmp_buf as conditional jmp_buf alternative */
#define HAVE_SIGJMP_BUF 1

/* Define if your OpenSSL library provides the SSL_CTX_GET0_PARAM function */
#define HAVE_SSL_CTX_GET0_PARAM 1

/* Define if your OpenSSL library provides the RAND_egd function */
#define HAVE_RAND_EGD 1

/* Always define STDIO_NAMESPACE to ::, because MSVC6 gets mad if you don't. */
#define STDIO_NAMESPACE ::

/* Define if we can use C++11 */
/* #undef HAVE_CXX11 */

#if defined(HAVE_CXX11) && defined(__cplusplus) && __cplusplus < 201103L
#error\
DCMTK was configured to use C++11 features, but your compiler does not or was not configured to provide them.
#endif

/* Define if we can use C++14 */
/* #undef HAVE_CXX14 */

#if defined(HAVE_CXX14) && defined(__cplusplus) && __cplusplus < 201402L
#error\
DCMTK was configured to use C++14 features, but your compiler does not or was not configured to provide them.
#endif

/* Define if we can use C++17 */
/* #undef HAVE_CXX17 */

#if defined(HAVE_CXX17) && defined(__cplusplus) && __cplusplus < 201703L
#error\
DCMTK was configured to use C++17 features, but your compiler does not or was not configured to provide them.
#endif

/* Define if the compiler supports __alignof__ */
#define HAVE_GNU_ALIGNOF 1

/* Define if the compiler supports __alignof */
#define HAVE_MS_ALIGNOF 1

/* Define if the compiler supports __attribute__((aligned)) */
#define HAVE_ATTRIBUTE_ALIGNED 1

/* Define if __attribute__((aligned)) supports templates */
#define ATTRIBUTE_ALIGNED_SUPPORTS_TEMPLATES 1

/* Define if the compiler supports __declspec(align) */
/* #undef HAVE_DECLSPEC_ALIGN */

/* Define if the compiler supports default constructor detection via SFINAE */
#define HAVE_DEFAULT_CONSTRUCTOR_DETECTION_VIA_SFINAE 1

/* Define if we are cross compiling */
/* #undef DCMTK_CROSS_COMPILING */

/* The path on the Android device that should be used for temporary files */
/* #undef ANDROID_TEMPORARY_FILES_LOCATION */

/* Define if we are supposed to use STL's vector */
/* #undef HAVE_STL_VECTOR */

/* Define if we are supposed to use STL's algorithms */
/* #undef HAVE_STL_ALGORITHM */

/* Define if we are supposed to use STL's limit */
/* #undef HAVE_STL_LIMITS */

/* Define if we are supposed to use STL's list */
/* #undef HAVE_STL_LIST */

/* Define if we are supposed to use STL's list */
/* #undef HAVE_STL_MAP */

/* Define if we are supposed to use STL's memory */
/* #undef HAVE_STL_MEMORY */

/* Define if we are supposed to use STL's stack */
/* #undef HAVE_STL_STACK */

/* Define if we are supposed to use STL's string */
/* #undef HAVE_STL_STRING */

/* Define if we are supposed to use STL's type_traits */
/* #undef HAVE_STL_TYPE_TRAITS */

/* Define if we are supposed to use STL's tuple */
/* #undef HAVE_STL_TUPLE */

/* Define if we are supposed to use STL's system_error */
/* #undef HAVE_STL_SYSTEM_ERROR */

/* Define if the input iterator category is supported */
#define HAVE_INPUT_ITERATOR_CATEGORY 1

/* Define if the input iterator category is supported */
#define HAVE_OUTPUT_ITERATOR_CATEGORY 1

/* Define if the input iterator category is supported */
#define HAVE_FORWARD_ITERATOR_CATEGORY 1

/* Define if the input iterator category is supported */
#define HAVE_BIDIRECTIONAL_ITERATOR_CATEGORY 1

/* Define if the input iterator category is supported */
#define HAVE_RANDOM_ACCESS_ITERATOR_CATEGORY 1

/* Define if the input iterator category is supported */
/* #undef HAVE_CONTIGUOUS_ITERATOR_CATEGORY */

#endif /* !OSCONFIG_H*/
