add_library(zbus-master alert.h alert.c cmd.h cmd.c bus.c bus.h)
add_executable(zbus main.c)
target_link_libraries(zbus zbus-master)
