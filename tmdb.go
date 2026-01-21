package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"tmdb_scraper/models"
)

type Usecase struct {
	tmdbApiBaseUrl string
	client         *HttpClient
}

func NewUsecase(tmdbApiBaseUrl string, client *HttpClient) *Usecase {
	return &Usecase{
		tmdbApiBaseUrl: tmdbApiBaseUrl,
		client:         client,
	}
}

func (u *Usecase) GetMovieDetails(id string, at string) (models.TMDBMovie, error) {
	var response models.TMDBMovie
	url := fmt.Sprintf(
		"%s/movie/%s?append_to_response=credits,images,external_ids,similar,belongs_to_collection",
		u.tmdbApiBaseUrl, id,
	)

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add(
		"Authorization",
		fmt.Sprintf("Bearer %s", at),
	)

	res, err := u.client.Do(req)
	if err != nil {
		fmt.Println("Error sending get movie request to TMDB", err)
		return response, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		fmt.Println("Invalid status code from get movie request to TMDB", res.StatusCode)
		return response, fmt.Errorf("Getting invalid status code %d for %s", res.StatusCode, id)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading response of get movie request to TMDB", err)
		return response, err
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println("Error unmarshalling get movie response", err)
		return response, err
	}

	if response.BelongsToCollection.ID != 0 {

		url = fmt.Sprintf(
			"%s/collection/%d",
			u.tmdbApiBaseUrl,
			response.BelongsToCollection.ID,
		)

		req, _ = http.NewRequest("GET", url, nil)

		req.Header.Add("accept", "application/json")
		req.Header.Add(
			"Authorization",
			fmt.Sprintf("Bearer %s", at),
		)

		res, err = u.client.Do(req)
		if err != nil {
			fmt.Println("Error sending get collection request to TMDB", err)
			return response, err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			fmt.Println("Invalid status code from get collection request to TMDB", res.StatusCode)
			return response, err
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			fmt.Println("Error reading response of get collection request to TMDB", err)
			return response, err
		}

		var collection models.Collection
		err = json.Unmarshal(body, &collection)
		if err != nil {
			fmt.Println("Error unmarshalling get collection response", err)
			return response, err
		}
		response.Collection = collection
	}

	return response, nil
}

func (u *Usecase) GetShowDetails(id string, at string) (models.TMDBShow, error) {
	var details models.TMDBShow
	url := fmt.Sprintf(
		"%s/tv/%s?append_to_response=credits,external_ids,images,similar",
		u.tmdbApiBaseUrl, id,
	)

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add(
		"Authorization",
		fmt.Sprintf("Bearer %s", at),
	)

	res, err := u.client.Do(req)
	if err != nil {
		fmt.Println("Error sending get series request to TMDB", err)
		return details, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading response of get series request to TMDB", err)
		return details, err
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(
			"Invalid status code from get series request to TMDB",
			res.StatusCode,
			string(body),
		)
		return details, fmt.Errorf("Got invalid status code %d for %s", res.StatusCode, id)
	}

	err = json.Unmarshal(body, &details)
	if err != nil {
		fmt.Println("Error unmarshalling show response", err, string(body))
		return details, err
	}

	var seasons []models.Season

	seasonInOneIteration := 10
	iterations := int(math.Ceil((float64(len(details.Seasons)) / float64(seasonInOneIteration))))
	for iteration := range iterations {
		keys := []string{}
		url = fmt.Sprintf(
			"%s/tv/%s?append_to_response=",
			u.tmdbApiBaseUrl, id,
		)
	inner:
		for i := range seasonInOneIteration {
			index := i + (seasonInOneIteration * iteration)
			maxIndex := (seasonInOneIteration - 1) + (seasonInOneIteration * iteration)
			if index >= len(details.Seasons) {
				break inner
			}
			currSeason := details.Seasons[index]
			if currSeason.SeasonNumber == 0 {
				continue inner
			}
			seasonKey := fmt.Sprintf("season/%d", currSeason.SeasonNumber)
			url += seasonKey
			keys = append(keys, seasonKey)
			if index != maxIndex && maxIndex != len(details.Seasons)-1 {
				url += ","
			}
		}
		req, _ = http.NewRequest("GET", url, nil)

		req.Header.Add("accept", "application/json")
		req.Header.Add(
			"Authorization",
			fmt.Sprintf("Bearer %s", at),
		)

		res, err = u.client.Do(req)
		if err != nil {
			fmt.Println("Error sending get series request to TMDB", err)
			return details, err
		}

		defer res.Body.Close()
		body, err = io.ReadAll(res.Body)
		if err != nil {
			fmt.Println("Error reading response of get series request to TMDB", err)
			return details, err
		}

		if res.StatusCode != http.StatusOK {
			fmt.Println(
				"Invalid status code from get series request to TMDB",
				res.StatusCode,
				string(body),
			)
			return details, fmt.Errorf("Got invalid status code %d for  %s", res.StatusCode, id)
		}

		rawMap := make(map[string]json.RawMessage, 0)
		err = json.Unmarshal(body, &rawMap)
		if err != nil {
			return details, fmt.Errorf(
				"Error unmarshalling season response %v %s\n",
				err,
				string(body),
			)
		}
		for _, k := range keys {
			var temp models.Season
			err = json.Unmarshal(rawMap[k], &temp)
			if err != nil {
				fmt.Println("Error unmarshalling show season response", err)
			return details, fmt.Errorf(
				"Error unmarshalling season response %v %s\n",
				err,
				string(rawMap[k]),
			)
			}
			seasons = append(seasons, temp)
		}
	}

	details.Seasons = seasons

	return details, nil
}
