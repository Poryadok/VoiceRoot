/* Isolated PostgreSQL test fixture only. No kernel or monotonic clock changes.
 * A positive decimal Unix microsecond value freezes this process's wall clock;
 * zero/missing resets real wall time. Never load this library in a deployment. */
#define _GNU_SOURCE
#include <dlfcn.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdlib.h>
#include <sys/time.h>
#include <time.h>
#include <unistd.h>

static int64_t fixture_wall_us(void) {
    int fd = open("/fixture-clock/walltime-us", O_RDONLY | O_CLOEXEC);
    if (fd < 0) return 0;
    char text[64];
    ssize_t size = read(fd, text, sizeof(text) - 1);
    close(fd);
    if (size <= 0) return 0;
    text[size] = '\0';
    char *end;
    int64_t value = strtoll(text, &end, 10);
    if (end == text || value <= 0 || (*end != '\0' && *end != '\n')) return 0;
    return value;
}

int gettimeofday(struct timeval *restrict value, void *restrict zone) {
    static int (*next)(struct timeval *, void *);
    if (!next) next = dlsym(RTLD_NEXT, "gettimeofday");
    int result = next(value, zone);
    int64_t wall = fixture_wall_us();
    if (result == 0 && wall > 0) {
        value->tv_sec = wall / 1000000;
        value->tv_usec = wall % 1000000;
    }
    return result;
}

int clock_gettime(clockid_t clock, struct timespec *value) {
    static int (*next)(clockid_t, struct timespec *);
    if (!next) next = dlsym(RTLD_NEXT, "clock_gettime");
    int result = next(clock, value);
    /* CLOCK_MONOTONIC and all timeout clocks stay real. */
    int64_t wall = clock == CLOCK_REALTIME ? fixture_wall_us() : 0;
    if (result == 0 && wall > 0) {
        value->tv_sec = wall / 1000000;
        value->tv_nsec = (wall % 1000000) * 1000;
    }
    return result;
}

time_t time(time_t *value) {
    static time_t (*next)(time_t *);
    if (!next) next = dlsym(RTLD_NEXT, "time");
    int64_t wall = fixture_wall_us();
    if (wall <= 0) return next(value);
    time_t result = wall / 1000000;
    if (value) *value = result;
    return result;
}
