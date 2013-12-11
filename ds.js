var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
    },

    getFightCard: function(msg) {
        $.get("/api/f").done(function(data){
            var common = DS.Web.findCommonOpponents(data.History);
            var msg = data.Alert;
            var state = $.trim(msg).length === 0;

            DS.Web.populateStats(data.Stats);
            DS.Web.populateData($(".red"), data.History[0], common);
            DS.Web.populateData($(".blue"), data.History[1], common);
            $('.msg').text(msg);
            $('.salty-alert').toggleClass('hidden', state);
        });
    },

    findCommonOpponents: function(data) {        
        var mapFunc = function(obj, index) {
            var a = (data[index].Wins || []).concat(data[index].Losses || []);
            return $.map(a, function (e, i){
                return e.Opponent;
            });
        };
        var ro = mapFunc(data, 0);
        var bo = mapFunc(data, 1);

        var result = $.grep(ro, function(e){
            return $.inArray(e, bo) !== -1;
        });

        return result;
    },

    populateStats: function(stats) {
        $('.red .tier').text(stats.p1tier);
        $('.red .life').text(stats.p1life);
        $('.red .meter').text(stats.p1meter);
        $('.blue .tier').text(stats.p2tier);
        $('.blue .life').text(stats.p2life);
        $('.blue .meter').text(stats.p2meter);        
    },

    populateData: function($elm, data, common) {
        data.Wins = data.Wins || [];
        data.Losses = data.Losses || [];

        $elm.find(".name").text(data.Fighter.Name);
        $elm.find(".elo").text(data.Fighter.Elo);
        var $tblW = $elm.find("table.wins tbody").empty();
        var $tblL = $elm.find("table.losses tbody").empty();        

        $($tblW.closest('.fights').find('thead th')[0]).text(data.Wins.length + ' Wins');
        $($tblL.closest('.fights').find('thead th')[1]).text(data.Losses.length + ' Losses');
        var appendFunc = function(a, $t) {
            $(a).sort(DS.Web.eloSort).each(function(index, item){
                DS.Web.appendRow($t, index, item, common);
            });
        };

        appendFunc(data.Wins, $tblW);
        appendFunc(data.Losses, $tblL);
    },

    appendRow: function($tbl, index, item, common) {
        var $row = $('<tr><td>' + item.Elo + '</td><td>' + item.Opponent + '</td></tr>');        
        if($.inArray(item.Opponent, common) !== -1) {
            $row.addClass('alert alert-info');
        }
        $tbl.append($row);
    },

    eloSort: function(a, b) {
        return a.Elo == b.Elo ? 0 : (a.Elo > b.Elo) ? -1 : 1;
    }
};

$(function(){
    DS.Web.init();
});
