var DS = DS || {};

DS.Search = {
    init: function(){
        $.get("/api/a").done(DS.Search.bindData);
        DS.Search.parseHash();
    },

    parseHash: function() {
        var ids = $(window.location.hash.substring(1).split('/')).map(function(idx, n) { return parseInt(n); });
        if(ids.length === 0) { 
            return; 
        }
        if(ids[0] > 0) {
            DS.Search.fetchFighter($('.red'), ids[0]);
        }
        if(ids[1] > 0) {
            DS.Search.fetchFighter($('.blue'), ids[1]);
        }
    },

    setHash: function() {
        var hash = $('input.name').map(function(idx, e) { return $(e).attr('data-cid'); }).get().join('/');
        window.location.hash = hash;
    },

    bindData: function(data) {
        $("input.name")
            .bind("typeahead:selected", function(e, o){
                var result = data.filter(function(c) {
                    return o.value == c.Name;
                });                
                var $elm = $(e.target).closest('.col-lg-6');

                if(result.length !== 0) {                                        
                    DS.Search.fetchFighter($elm, result[0].Cid);
                }
            })
            .typeahead({
                local: data.map(function(c) { return c.Name; })
            });
    },

    fetchFighter: function($elm, cid) {
        $.get("/api/h/" + cid).done(function(hist){
            $elm.find('input.name').attr('data-cid', cid);
            DS.Search.populateData($elm, hist);
        });
    },

    populateData: function($elm, data) {
        data.Wins = data.Wins || [];
        data.Losses = data.Losses || [];
        var tiers = {1:'S', 2:'A', 3:'B', 4:'P'};

        $elm.find(".name").val(data.Fighter.Name);
        $elm.find(".elo").text(data.Fighter.Elo);
        $elm.find(".tier").text(tiers[data.Fighter.Tier]);
        var $tblW = $elm.find("table.wins tbody").empty();
        var $tblL = $elm.find("table.losses tbody").empty();        

        $($tblW.closest('.fights').find('thead th')[0]).text(data.Wins.length + ' Wins');
        $($tblL.closest('.fights').find('thead th')[1]).text(data.Losses.length + ' Losses');
        var appendFunc = function(a, $t) {
            $(a).sort(DS.Search.eloSort).each(function(index, item){
                DS.Search.appendRow($t, index, item);
            });
        };

        appendFunc(data.Wins, $tblW);
        appendFunc(data.Losses, $tblL);
        DS.Search.setHash();
    },

    appendRow: function($tbl, index, item) {
        var $row = $('<tr><td>' + item.Elo + '</td><td>' + item.Opponent + '</td></tr>');
        $tbl.append($row);
    },    

    eloSort: function(a, b) {
        return a.Elo == b.Elo ? 0 : (a.Elo > b.Elo) ? -1 : 1;
    }
};

$(function(){
    DS.Search.init();
});