var gulp     = require('gulp'),
    util     = require('gulp-util'),
    notifier = require('node-notifier'),
    sync     = require('gulp-sync')(gulp).sync,
    reload   = require('gulp-livereload'),
    child    = require('child_process'),
    os       = require('os');

var server = null;

gulp.task('server:build', function() {
    var build = child.spawnSync('go', ['build']);

    if (build.stderr.length) {
        util.log(util.colors.red('Something wrong with this version: '));
        var lines = build.stderr.toString()
            .split('\n')
            .filter(function(line) {
                return line.length
            });

        for (var l in lines) {
            util.log(util.colors.red('Error (go build):' + lines[l]));
        }

        notifier.notify({
            title: 'Error (go build)',
            message: lines
        });
    }

    return build;
});

gulp.task('server:spawn', function() {
    if (server && server != 'null') {
        server.kill();
    }

    if (os.platform() == 'win32') {
        var path_folder = __dirname.split('\\');
    } else {
        var path_folder = __dirname.split('/');
    }

    var length = path_folder.length;
    var app    = './' + path_folder[length - parseInt(1)];

    if (os.platform() == 'win32') {
        app = app + '.exe';
    }

    server = child.spawn(app)
        .on('error', function(err) {
            console.log(err);
            return;
        });

    server.stderr.on('data', function(data) {
        process.stdout.write(data.toString());
    });
});

gulp.task('server:watch', function() {
    gulp.watch([
        '*.go',
        '**/*.go',
    ], sync([
        'server:build',
        'server:spawn'
    ], 'server'));
});

gulp.task('default', ['server:build', 'server:spawn', 'server:watch']);
