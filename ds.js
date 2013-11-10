var DS = DS || {};

DS.Web = {
    init: function() {
        var socket = io.connect("http://www-cdn-twitch.saltybet.com:8000");
        socket.on("message", this.getFightCard);
    },

    getFightCard: function(msg) {
        $.get("/api/f").done(function(data){
            var common = DS.Web.findCommonOpponents(data);
            DS.Web.populateData($(".red"), data[0], common);
            DS.Web.populateData($(".blue"), data[1], common);
        });
    },

    findCommonOpponents: function(data) {        
        var mapFunc = function(obj, index) {
            var a = (data[index].Wins || []).concat(data[index].Losses || []);
            return $.map(a, function (e, i){
                return e.Opponent;
            });
        }
        var ro = mapFunc(data, 0);
        var bo = mapFunc(data, 1);

        var result = $.grep(ro, function(e){
            return $.inArray(e, bo) !== -1;
        });

        return result;
    },

    populateData: function($elm, data, common) {
        data.Wins = data.Wins || [];
        data.Losses = data.Losses || [];

        $elm.find(".name").text(data.Fighter.Name);
        $elm.find(".label").text(data.Fighter.Elo);
        var $tblW = $elm.find("table.wins tbody").empty();
        var $tblL = $elm.find("table.losses tbody").empty();        

        $($tblW.closest('.fights').find('thead th')[0]).text(data.Wins.length + ' Wins');
        $($tblL.closest('.fights').find('thead th')[1]).text(data.Losses.length + ' Losses');

        $(data.Wins).sort(DS.Web.eloSort).each(function(index, item){
            DS.Web.appendRow($tblW, index, item, common);
        });
        $(data.Losses).sort(DS.Web.eloSort).each(function(index, item){
            DS.Web.appendRow($tblL, index, item, common);
        });        
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

$(document).ready(function(){
    DS.Web.init();
});